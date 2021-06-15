package main

import (
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go/v2" // make sure to use v2 cloudevents here
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"time"
)

/**
* Here are all the handler functions for the individual event
* See https://github.com/keptn/spec/blob/0.8.0-alpha/cloudevents.md for details on the payload
**/

const (
	ResourcePrefix = "gatling"
	ConfFilename   = "gatling.conf.yaml"
)

// HandleTestTriggeredEvent handles test.triggered events
func HandleTestTriggeredEvent(myKeptn *keptnv2.Keptn, incomingEvent cloudevents.Event, data *keptnv2.TestTriggeredEventData) error {
	log.Infof("Handling test.triggered Event: %s", incomingEvent.Context.GetID())

	// Send out a test.started CloudEvent
	_, err := myKeptn.SendTaskStartedEvent(&keptnv2.EventData{}, ServiceName)
	if err != nil {
		log.Errorf("Failed to send task started CloudEvent (%s), aborting... \n", err.Error())
		return err
	}

	serviceURL, err := getServiceURL(data)
	if err != nil {
		return erroredTestsFinishedEvent(myKeptn, err)
	}

	// create a tempdir
	tempDir, err := ioutil.TempDir("", ResourcePrefix)
	if err != nil {
		return erroredTestsFinishedEvent(myKeptn, err)
	}

	// cleanup afterwards
	defer os.RemoveAll(tempDir)

	err = getAllGatlingResources(myKeptn, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService(), tempDir)
	if err != nil {
		err = fmt.Errorf("error loading %s/* files for %s.%s.%s: %s", ResourcePrefix, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService(), err.Error())
		return erroredTestsFinishedEvent(myKeptn, err)
	}
	err = restoreDefaultConfFiles(tempDir)
	if err != nil {
		err = fmt.Errorf("error syncing default conf files for %s.%s.%s: %s", myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService(), err.Error())
		return erroredTestsFinishedEvent(myKeptn, err)
	}
	var conf *GatlingConf
	conf, err = getGatlingConf(myKeptn, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService())
	if err != nil {
		log.Warnf("Failed to load Configuration file: %s - proceeding with default values", err.Error())
	}

	simulation := determineSimulationName(data, conf)

	log.Infof("TestStrategy=%s -> simulation=%s -> serviceUrl=%s\n", data.Test.TestStrategy, simulation, serviceURL.String())

	// CAPTURE START TIME
	startTime := time.Now()

	// -> https://github.com/keptn/keptn/blob/069dd0f5c7b6f37a3737f4c0c9c7cf07a801b039/jmeter-service/jmeterUtils.go#L184
	command := []string{
		fmt.Sprintf("--simulation=%s", simulation),
	}

	environment := os.Environ()
	environment = append(environment, fmt.Sprintf("GATLING_HOME=%s", tempDir))
	environment = append(environment, fmt.Sprintf("JAVA_OPTS=-DserviceURL=%s", serviceURL.String()))
	log.Info("Running gatling tests")
	str, err := ExecuteCommandWithEnv("gatling.sh", command, environment)

	log.Infof("Finished running gatling tests")
	log.Infof(str)

	if err != nil {
		return erroredTestsFinishedEvent(myKeptn, err)
	}

	endTime := time.Now()
	// Done

	finishedEvent := &keptnv2.TestFinishedEventData{
		Test: keptnv2.TestFinishedDetails{
			Start: startTime.Format(time.RFC3339),
			End:   endTime.Format(time.RFC3339),
		},
		EventData: keptnv2.EventData{
			Result:  keptnv2.ResultPass,
			Status:  keptnv2.StatusSucceeded,
			Message: "Gatling test finished successfully",
		},
	}

	// Finally: send out a test.finished CloudEvent
	_, err = myKeptn.SendTaskFinishedEvent(finishedEvent, ServiceName)

	if err != nil {
		return erroredTestsFinishedEvent(myKeptn, err)
	}

	return nil
}

func erroredTestsFinishedEvent(myKeptn *keptnv2.Keptn, err error) error {
	if eventErr := sendErroredTestsFinishedEvent(myKeptn, err); eventErr != nil {
		log.Errorf(fmt.Sprintf("Error sending test finished event: %s", eventErr.Error()))
	}
	return err
}

func sendErroredTestsFinishedEvent(myKeptn *keptnv2.Keptn, err error) error {
	// report error
	log.Error(err)
	// send out a test.finished failed CloudEvent
	_, err = myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
		Status:  keptnv2.StatusErrored,
		Result:  keptnv2.ResultFailed,
		Message: err.Error(),
	}, ServiceName)
	return err
}
