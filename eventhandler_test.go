package main

import (
	"encoding/json"
	"fmt"
	"github.com/keptn/go-utils/pkg/lib/v0_2_0/fake"
	"io/ioutil"
	"strings"
	"testing"

	keptn "github.com/keptn/go-utils/pkg/lib/keptn"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"

	cloudevents "github.com/cloudevents/sdk-go/v2" // make sure to use v2 cloudevents here
)

/**
 * loads a cloud event from the passed test json file and initializes a keptn object with it
 */
func initializeTestObjects(eventFileName string) (*keptnv2.Keptn, *cloudevents.Event, error) {
	// load sample event
	eventFile, err := ioutil.ReadFile(eventFileName)
	if err != nil {
		return nil, nil, fmt.Errorf("Cant load %s: %s", eventFileName, err.Error())
	}

	incomingEvent := &cloudevents.Event{}
	err = json.Unmarshal(eventFile, incomingEvent)
	if err != nil {
		return nil, nil, fmt.Errorf("Error parsing: %s", err.Error())
	}

	// Add a Fake EventSender to KeptnOptions
	var keptnOptions = keptn.KeptnOpts{
		EventSender: &fake.EventSender{},
	}
	keptnOptions.UseLocalFileSystem = true
	myKeptn, err := keptnv2.NewKeptn(incomingEvent, keptnOptions)

	return myKeptn, incomingEvent, err
}

/*
func TestHandleTestTriggeredEvent(t *testing.T) {
	myKeptn, incomingEvent, err := initializeTestObjects("test-events/test.triggered.json")
	if err != nil {
		t.Error(err)
		return
	}

	specificEvent := &keptnv2.TestTriggeredEventData{}
	err = incomingEvent.DataAs(specificEvent)
	if err != nil {
		t.Errorf("Error getting keptn event data")
	}

	err = HandleTestTriggeredEvent(myKeptn, *incomingEvent, specificEvent)
	if err != nil {
		t.Errorf("Error: " + err.Error())
	}

	gotEvents := len(myKeptn.EventSender.(*fake.EventSender).SentEvents)

	// Verify that HandleTestTriggeredEvent has sent 2 cloudevents
	if gotEvents != 2 {
		t.Errorf("Expected two events to be sent, but got %v", gotEvents)
	}

	// Verify that the first CE sent is a .started event
	if keptnv2.GetStartedEventType(keptnv2.TestTaskName) != myKeptn.EventSender.(*fake.EventSender).SentEvents[0].Type() {
		t.Errorf("Expected a test.started event type")
	}

	// Verify that the second CE sent is a .finished event
	if keptnv2.GetFinishedEventType(keptnv2.TestTaskName) != myKeptn.EventSender.(*fake.EventSender).SentEvents[1].Type() {
		t.Errorf("Expected a test.finished event type")
	}
}
*/

func TestGetServiceURL(t *testing.T) {
	inputUrl := "http://some.host.name:8021"
	t.Run("Local Deployment", func(t *testing.T) {

		data := &keptnv2.TestTriggeredEventData{
			Deployment: keptnv2.TestTriggeredDeploymentDetails{
				DeploymentURIsLocal: strings.Split(inputUrl, ";"),
			},
		}
		url, err := getServiceURL(data)
		if err != nil {
			t.Errorf("Failed %v", err)
		}
		if url.String() != inputUrl {
			t.Errorf("Expected: %s got: %s", inputUrl, url)
		}
	})
	t.Run("Public Deployment", func(t *testing.T) {
		data := &keptnv2.TestTriggeredEventData{
			Deployment: keptnv2.TestTriggeredDeploymentDetails{
				DeploymentURIsPublic: strings.Split(inputUrl, ";"),
			},
		}
		url, err := getServiceURL(data)
		if err != nil {
			t.Errorf("Failed %v", err)
		}
		if url.String() != inputUrl {
			t.Errorf("Expected: %s got: %s", inputUrl, url)
		}
	})
	t.Run("Missing Deployment config", func(t *testing.T) {
		data := &keptnv2.TestTriggeredEventData{}
		_, err := getServiceURL(data)
		if err == nil {
			t.Errorf("Expected an error")
		}
	})
}

func TestDetermineSimulationName(t *testing.T) {
	data := &keptnv2.TestTriggeredEventData{
		Test: keptnv2.TestTriggeredDetails{
			TestStrategy: "custom_test",
		},
	}

	t.Run("Missing config", func(t *testing.T) {
		simulation := determineSimulationName(data, nil)
		if simulation != "CustomTestSimulation" {
			t.Errorf("Expcted CustomTestSimulation got %s", simulation)
		}
	})

	t.Run("Simple config", func(t *testing.T) {
		conf := &GatlingConf{
			Workloads: []*Workload{
				{
					TestStrategy: "custom_test",
					Simulation:   "RandomSimulation",
				},
				{
					TestStrategy: "custom_test",
					Simulation:   "UnusedSimulation",
				},
			},
		}

		simulation := determineSimulationName(data, conf)
		if simulation != "RandomSimulation" {
			t.Errorf("Expcted CustomTestSimulation got %s", simulation)
		}
	})
}
