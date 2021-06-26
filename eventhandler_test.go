package main

import (
	"encoding/json"
	"fmt"
	"github.com/keptn/go-utils/pkg/lib/v0_2_0/fake"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	keptn "github.com/keptn/go-utils/pkg/lib/keptn"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"

	cloudevents "github.com/cloudevents/sdk-go/v2" // make sure to use v2 cloudevents here
	keptnapimodels "github.com/keptn/go-utils/pkg/api/models"
)

/**
 * loads a cloud event from the passed test json file and initializes a keptn object with it
 */
func initializeTestObjects(configurationServiceURL string, eventFileName string) (*keptnv2.Keptn, *cloudevents.Event, error) {
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
		ConfigurationServiceURL: configurationServiceURL,
		EventSender: &fake.EventSender{},
	}
	keptnOptions.UseLocalFileSystem = true
	myKeptn, err := keptnv2.NewKeptn(incomingEvent, keptnOptions)

	return myKeptn, incomingEvent, err
}

func initializeTestServer(returnedResources keptnapimodels.Resources, returnedStatus int) *httptest.Server {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			log.Warnf("Request path %s", r.URL.Path)
			if strings.HasSuffix(r.URL.Path, "/resource/") {
				marshal, _ := json.Marshal(returnedResources)
				w.Write(marshal)
				return
			}

			w.WriteHeader(returnedStatus)
			w.Write([]byte(`{
	"code": ` + fmt.Sprintf("%d", returnedStatus) + `,
	"message": ""
}`))
		}),
	)
	return ts
}

func TestHandleTestTriggeredEventSkipIfConfigMissing(t *testing.T) {

	returnedResources := keptnapimodels.Resources{
		Resources: []*keptnapimodels.Resource{},
	}
	ts := initializeTestServer(returnedResources, http.StatusNotFound)
	defer ts.Close()

	myKeptn, incomingEvent, err := initializeTestObjects(ts.URL,"test-events/test.triggered.json")
	if err != nil {
		t.Error(err)
		return
	}

	specificEvent := &keptnv2.TestTriggeredEventData{}
	err = incomingEvent.DataAs(specificEvent)
	if err != nil {
		t.Errorf("Error getting keptn event data")
	}

	executionHandler := func (args []string, env []string) (string, error) {
		t.Errorf("Unexpected execution call")
		return "", nil
	}

	err = HandleTestTriggeredEvent(myKeptn, executionHandler, *incomingEvent, specificEvent)
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

	sentEvent := &keptnv2.TestTriggeredEventData{}
	err = myKeptn.EventSender.(*fake.EventSender).SentEvents[1].DataAs(sentEvent)
	if err != nil {
		t.Errorf("Error getting keptn event data")
	}
	if sentEvent.Message != "Gatling test skipped" {
		t.Errorf("Expected skipped test got: %s", sentEvent.Message)
	}
}

func TestHandleTestTriggeredEventFailures(t *testing.T) {

	returnedResources := keptnapimodels.Resources{
		Resources: []*keptnapimodels.Resource{},
	}
	ts := initializeTestServer(returnedResources, http.StatusNotFound)
	defer ts.Close()

	myKeptn, incomingEvent, err := initializeTestObjects(ts.URL,"test-events/test.triggered.no-uris.json")
	if err != nil {
		t.Error(err)
		return
	}

	specificEvent := &keptnv2.TestTriggeredEventData{}
	err = incomingEvent.DataAs(specificEvent)
	if err != nil {
		t.Errorf("Error getting keptn event data")
	}

	executionHandler := func (args []string, env []string) (string, error) {
		t.Errorf("Unexpected execution call")
		return "", nil
	}

	err = HandleTestTriggeredEvent(myKeptn, executionHandler, *incomingEvent, specificEvent)
	if err != nil && err.Error() != "no deployment URI included in event" {
		t.Errorf("Unexpected Error: " + err.Error())
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

	sentEvent := &keptnv2.TestTriggeredEventData{}
	err = myKeptn.EventSender.(*fake.EventSender).SentEvents[1].DataAs(sentEvent)
	if err != nil {
		t.Errorf("Error getting keptn event data")
	}
	if sentEvent.Result != keptnv2.ResultFailed {
		t.Errorf("Expected failed event")
	}
	if sentEvent.Message != "no deployment URI included in event" {
		t.Errorf("Expected skipped test got: %s", sentEvent.Message)
	}
}

func TestHandleTestTriggeredEventSimpleTest(t *testing.T) {
	contentUri := "test-data/gatling/user-files/simulations/SomeSimulation.scala"
	returnedResources := keptnapimodels.Resources{
		Resources: []*keptnapimodels.Resource{
			{
				ResourceContent: "empty",
				ResourceURI:     &contentUri,
			},
		},
	}
	ts := initializeTestServer(returnedResources, http.StatusOK)
	defer ts.Close()

	myKeptn, incomingEvent, err := initializeTestObjects(ts.URL, "test-events/test.triggered.json")
	if err != nil {
		t.Error(err)
		return
	}

	specificEvent := &keptnv2.TestTriggeredEventData{}
	err = incomingEvent.DataAs(specificEvent)
	if err != nil {
		t.Errorf("Error getting keptn event data")
	}

	executionHandler := func(args []string, env []string) (string, error) {
		if len(args) != 1 {
			t.Errorf("Unexpected execution arguments")
		}

		if args[0] != "--simulation=SomeSimulation" {
			t.Errorf("Unexpected simulation argument got %s", args[0])
		}

		return "", nil
	}

	err = HandleTestTriggeredEvent(myKeptn, executionHandler, *incomingEvent, specificEvent)
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

	sentEvent := &keptnv2.TestTriggeredEventData{}
	err = myKeptn.EventSender.(*fake.EventSender).SentEvents[1].DataAs(sentEvent)
	if err != nil {
		t.Errorf("Error getting keptn event data")
	}
	if sentEvent.Message != "Gatling test finished successfully" {
		t.Errorf("Expected successful test got: %s", sentEvent.Message)
	}
}

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
