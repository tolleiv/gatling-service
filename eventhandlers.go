package main

import (
	"errors"
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go/v2" // make sure to use v2 cloudevents here
	"github.com/iancoleman/strcase"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
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

// GatlingConf Configuration file type
type GatlingConf struct {
	SpecVersion string      `json:"spec_version" yaml:"spec_version"`
	Workloads   []*Workload `json:"workloads" yaml:"workloads"`
}

// Workload of Keptn stage
type Workload struct {
	TestStrategy string `json:"teststrategy" yaml:"teststrategy"`
	Simulation   string `json:"simulation" yaml:"simulation"`
}

// HandleTestTriggeredEvent handles test.triggered events
func HandleTestTriggeredEvent(myKeptn *keptnv2.Keptn, incomingEvent cloudevents.Event, data *keptnv2.TestTriggeredEventData) error {
	log.Printf("Handling test.triggered Event: %s", incomingEvent.Context.GetID())

	// Send out a migrate.started CloudEvent
	// The get-sli.started cloud-event is new since Keptn 0.8.0 and is required to be send when the task is started
	_, err := myKeptn.SendTaskStartedEvent(&keptnv2.EventData{}, ServiceName)

	if err != nil {
		log.Printf("Failed to send task started CloudEvent (%s), aborting... \n", err.Error())
		return err
	}

	serviceURL, err := getServiceURL(data)
	if err != nil {
		if eventErr := sendErroredTestsFinishedEvent(myKeptn, err); eventErr != nil {
			log.Printf(fmt.Sprintf("Error sending test finished event: %s", eventErr.Error()))
		}
		return err
	}

	// create a tempdir
	tempDir, err := ioutil.TempDir("", ResourcePrefix)
	if err != nil {
		if eventErr := sendErroredTestsFinishedEvent(myKeptn, err); eventErr != nil {
			log.Printf(fmt.Sprintf("Error sending test finished event: %s", eventErr.Error()))
		}
		return err
	}

	// cleanup afterwards
	defer os.RemoveAll(tempDir)

	err = getAllGatlingResources(myKeptn, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService(), tempDir)
	if err != nil {
		err = fmt.Errorf("error loading %s/* files for %s.%s.%s: %s", ResourcePrefix, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService(), err.Error())
		if eventErr := sendErroredTestsFinishedEvent(myKeptn, err); eventErr != nil {
			log.Printf(fmt.Sprintf("Error sending test finished event: %s", eventErr.Error()))
		}
		return err
	}
	err = restoreDefaultConfFiles(tempDir)
	if err != nil {
		err = fmt.Errorf("error syncing default conf files for %s.%s.%s: %s", myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService(), err.Error())
		if eventErr := sendErroredTestsFinishedEvent(myKeptn, err); eventErr != nil {
			log.Printf(fmt.Sprintf("Error sending test finished event: %s", eventErr.Error()))
		}
		return err
	}
	var conf *GatlingConf
	conf, err = getGatlingConf(myKeptn, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService())

	if err != nil {
		log.Printf("Failed to load Configuration file: %s", err.Error())
	}

	var simulation = fmt.Sprintf("%sSimulation", strcase.ToCamel(data.Test.TestStrategy))
	if conf != nil {
		for _, workload := range conf.Workloads {
			if workload.TestStrategy == data.Test.TestStrategy {
				if workload.Simulation != "" {
					simulation = workload.Simulation
				}
			}
		}
	}

	log.Printf("TestStrategy=%s -> simulation=%s -> serviceUrl=%s\n", data.Test.TestStrategy, simulation, serviceURL.String())

	// CAPTURE START TIME
	startTime := time.Now()

	// -> https://github.com/keptn/keptn/blob/069dd0f5c7b6f37a3737f4c0c9c7cf07a801b039/jmeter-service/jmeterUtils.go#L184
	command := []string{
		fmt.Sprintf("--simulation=%s", simulation),
	}

	log.Println("Prepare environment")

	environment := os.Environ()
	environment = append(environment, fmt.Sprintf("GATLING_HOME=%s", tempDir))
	environment = append(environment, fmt.Sprintf("JAVA_OPTS=-DserviceURL=%s", serviceURL.String()))
	log.Println("Running gatling tests")
	str, err := ExecuteCommandWithEnv("gatling.sh", command, environment)

	log.Println("Finished running gatling tests")
	log.Println(str)

	if err != nil {
		if eventErr := sendErroredTestsFinishedEvent(myKeptn, err); eventErr != nil {
			log.Printf(fmt.Sprintf("Error sending test finished event: %s", eventErr.Error()))
		}
		return err
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
		log.Printf("Failed to send task finished CloudEvent (%s), aborting...\n", err.Error())
		return err
	}

	return nil
}

// Loads gatling.conf.yaml for the current service
func getGatlingConf(myKeptn *keptnv2.Keptn, project string, stage string, service string) (*GatlingConf, error) {
	var err error

	confFile := path.Join(ResourcePrefix, ConfFilename)
	log.Printf("Loading %s for %s.%s.%s", confFile, project, stage, service)

	keptnResourceContent, err := myKeptn.GetKeptnResource(confFile)

	if err != nil {
		logMessage := fmt.Sprintf("error when trying to load %s file for service %s on stage %s or project-level %s: %s", confFile, service, stage, project, err.Error())
		return nil, errors.New(logMessage)
	}
	if len(keptnResourceContent) == 0 {
		// if no configuration file is available, this is not an error, as the service will proceed with the default workload
		log.Printf("no %s found", confFile)
		return nil, nil
	}

	var gatlingConf *GatlingConf
	gatlingConf, err = parseGatlingConf(keptnResourceContent)
	if err != nil {
		logMessage := fmt.Sprintf("Couldn't parse %s file found for service %s in stage %s in project %s. Error: %s", confFile, service, stage, project, err.Error())
		return nil, errors.New(logMessage)
	}

	log.Printf("Successfully loaded %s with %d workloads", ConfFilename, len(gatlingConf.Workloads))

	return gatlingConf, nil
}

// parses content and maps it to the GatlingConf struct
func parseGatlingConf(input []byte) (*GatlingConf, error) {
	gatlingconf := &GatlingConf{}
	err := yaml.Unmarshal(input, &gatlingconf)
	if err != nil {
		return nil, err
	}
	return gatlingconf, nil
}

//
// returns the service URL that is either passed via the DeploymentURI* parameters or constructs one based on keptn naming structure
//
func getServiceURL(data *keptnv2.TestTriggeredEventData) (*url.URL, error) {
	if len(data.Deployment.DeploymentURIsLocal) > 0 && data.Deployment.DeploymentURIsLocal[0] != "" {
		return url.Parse(data.Deployment.DeploymentURIsLocal[0])

	} else if len(data.Deployment.DeploymentURIsPublic) > 0 && data.Deployment.DeploymentURIsPublic[0] != "" {
		return url.Parse(data.Deployment.DeploymentURIsPublic[0])
	}

	return nil, errors.New("no deployment URI included in event")
}

func sendErroredTestsFinishedEvent(myKeptn *keptnv2.Keptn, err error) error {
	// report error
	log.Print(err)
	// send out a test.finished failed CloudEvent
	_, err = myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
		Status:  keptnv2.StatusErrored,
		Result:  keptnv2.ResultFailed,
		Message: err.Error(),
	}, ServiceName)
	return err
}

func restoreDefaultConfFiles(tempDir string) error {
	targetConf := path.Join([]string{tempDir, "conf"}...)
	err := os.MkdirAll(targetConf, 700)
	if err != nil {
		return err
	}
	for _, file := range []string{"logback.xml", "gatling.conf", "gatling-akka.conf"} {
		sourceConf := path.Join([]string{"opt", "gatling", "conf", file}...)
		_, err = ExecuteCommandWithEnv("cp", []string{"-u", sourceConf, targetConf}, []string{})
		if err != nil {
			return err
		}
	}
	return nil
}
