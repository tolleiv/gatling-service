package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/keptn/go-utils/pkg/lib/v0_2_0/fake"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
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
		EventSender:             &fake.EventSender{},
	}
	//keptnOptions.UseLocalFileSystem = true
	myKeptn, err := keptnv2.NewKeptn(incomingEvent, keptnOptions)

	return myKeptn, incomingEvent, err
}

func initializeTestServer(returnedResources keptnapimodels.Resources, sourcePath string) *httptest.Server {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			log.Debugf("Request path %s", r.URL.Path)
			if strings.HasSuffix(r.URL.Path, "/resource/") {
				marshal, _ := json.Marshal(returnedResources)
				_, _ = w.Write(marshal)
				return
			}
			for _, resource := range returnedResources.Resources {
				if strings.HasSuffix(r.URL.Path, *resource.ResourceURI) {
					content, _ := ioutil.ReadFile(path.Join(sourcePath, *resource.ResourceURI))
					resource.ResourceContent = b64.StdEncoding.EncodeToString(content)
					marshal, _ := json.Marshal(resource)
					_, _ = w.Write(marshal)
					return
				}
			}

			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{
	"code": ` + fmt.Sprintf("%d", http.StatusNotFound) + `,
	"message": ""
}`))
		}),
	)
	return ts
}

func assetStartedAndFinishedEvents(t *testing.T, gotEvents int, myKeptn *keptnv2.Keptn) {
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

func TestHandleTestTriggeredEvent(t *testing.T) {

	// prepare fixtures
	contentUriSimple := "gatling/user-files/simulations/SomeSimulation.scala"
	resourcesSimple := []*keptnapimodels.Resource{
		{
			ResourceURI: &contentUriSimple,
		},
	}
	resourcesEmpty := []*keptnapimodels.Resource{}
	contentUriWithConfig := "gatling/user-files/simulations/PerformanceSimulation.scala"
	configUriWithConfig := "gatling/gatling.conf.yaml"
	resourcesWithConfig := []*keptnapimodels.Resource{
		{
			ResourceURI: &contentUriWithConfig,
		},
		{
			ResourceURI: &configUriWithConfig,
		},
	}

	// tests cases
	type test struct {
		name               string
		inputFile          string
		resourceSourcePath string
		resources          []*keptnapimodels.Resource
		executionHandler   GatlingExecutionHandler
		expectedResult     keptnv2.ResultType
		expectedMessage    string
	}

	tests := []test{
		{
			"Skip if config is missing",
			"test-events/test.triggered.json",
			"test-data/simple/",
			resourcesEmpty,
			nil,
			keptnv2.ResultPass,
			"Gatling test skipped",
		},
		{
			"Fail when deploymentUri is missing",
			"test-events/test.triggered.no-uris.json",
			"test-data/simple/",
			resourcesEmpty,
			nil,
			keptnv2.ResultFailed,
			"no deployment URI included in event",
		},
		{
			"Fail if execution doesn't succeed",
			"test-events/test.triggered.json",
			"test-data/simple/",
			resourcesSimple,
			func(args []string, env []string) (string, error) {
				return "", errors.New("execution failed")
			},
			keptnv2.ResultFailed,
			"execution failed",
		},
		{
			"Successful test run - simple",
			"test-events/test.triggered.json",
			"test-data/simple/",
			resourcesSimple,
			func(args []string, env []string) (string, error) {
				if len(args) != 1 {
					t.Errorf("Unexpected execution arguments")
				}

				if args[0] != "--simulation=SomeSimulation" {
					t.Errorf("Unexpected simulation argument got %s", args[0])
				}

				return "", nil
			},
			keptnv2.ResultPass,
			"Gatling test finished successfully",
		},
		{
			"Successful test run - with config",
			"test-events/test.triggered.json",
			"test-data/with-configuration/",
			resourcesWithConfig,
			func(args []string, env []string) (string, error) {
				if len(args) != 1 {
					t.Errorf("Unexpected execution arguments")
				}

				if args[0] != "--simulation=PerformanceSimulation" {
					t.Errorf("Unexpected simulation argument got %s", args[0])
				}

				return "", nil
			},
			keptnv2.ResultPass,
			"Gatling test finished successfully",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			returnedResources := keptnapimodels.Resources{
				Resources: testCase.resources,
			}
			ts := initializeTestServer(returnedResources, testCase.resourceSourcePath)
			defer ts.Close()

			myKeptn, incomingEvent, err := initializeTestObjects(ts.URL, testCase.inputFile)
			if err != nil {
				t.Error(err)
				return
			}

			specificEvent := &keptnv2.TestTriggeredEventData{}
			if err = incomingEvent.DataAs(specificEvent); err != nil {
				t.Error(err)
				return
			}

			executionHandler := func(args []string, env []string) (string, error) {
				t.Errorf("Unexpected execution call")
				return "", nil
			}
			if testCase.executionHandler != nil {
				executionHandler = testCase.executionHandler
			}

			g := EventHandler{
				confDirRoot:      path.Join([]string{"test-data", "dist"}...),
				tempPathPrefix:   "./test-tmp/",
				executionHandler: executionHandler,
				myKeptn: myKeptn,
			}

			err = g.HandleTestTriggeredEvent(*incomingEvent, specificEvent)
			if err != nil && testCase.expectedResult != keptnv2.ResultFailed && err.Error() != testCase.expectedMessage {
				t.Errorf("Unexpected Error: " + err.Error())
			}

			gotEvents := len(myKeptn.EventSender.(*fake.EventSender).SentEvents)

			assetStartedAndFinishedEvents(t, gotEvents, myKeptn)

			sentEvent := &keptnv2.TestTriggeredEventData{}
			if err = myKeptn.EventSender.(*fake.EventSender).SentEvents[1].DataAs(sentEvent); err != nil {
				t.Errorf("Error getting keptn event data")
				return
			}
			if sentEvent.Result != testCase.expectedResult {
				t.Errorf("Expected event result %s got: %s", testCase.expectedResult, sentEvent.Result)
			}
			if sentEvent.Message != testCase.expectedMessage {
				t.Errorf("Expected message %s got: %s", testCase.expectedMessage, sentEvent.Message)
			}
		})
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
