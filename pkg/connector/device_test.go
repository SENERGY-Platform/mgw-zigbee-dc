/*
 * Copyright (c) 2023 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/devicerepo"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/docker"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/mocks"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/resources"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/zigbee2mqtt"
	"github.com/SENERGY-Platform/models/go/models"
	paho "github.com/eclipse/paho.mqtt.golang"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func ExampleGetMissingDeviceTypeMessage() {
	fmt.Print(strings.TrimSpace(GetMissingDeviceTypeMessage(resources.DeviceInfoExample[1], func(device model.ZigbeeDeviceInfo) map[string]interface{} {
		return zigbee2mqtt.GetZigbeeMessageStruct(device, zigbee2mqtt.ZigbeeEventAccess)
	}, func(device model.ZigbeeDeviceInfo) map[string]interface{} {
		return zigbee2mqtt.GetZigbeeMessageStruct(device, zigbee2mqtt.ZigbeeSetAccess)
	})))

	//output:
	//missing zigbee device-type, please provide a device-type with:
	//ref: https://www.zigbee2mqtt.io/devices/9290012573A.html
	//attributes:
	//     - senergy/zigbee-dc = true
	//     - senergy/zigbee-vendor = Philips
	//     - senergy/zigbee-model = 9290012573A
	//services:
	//---------------
	//local-id: get
	//protocol: standard-connector
	//example output data:
	//{
	//     "brightness": 254,
	//     "color": {
	//         "hue": 42,
	//         "saturation": 42,
	//         "x": 42,
	//         "y": 42
	//     },
	//     "color_temp": 500,
	//     "color_temp_startup": 500,
	//     "linkquality": 255,
	//     "power_on_behavior": "off",
	//     "state": "ON"
	//}
	//---------------
	//local-id: set
	//protocol: standard-connector
	//example input data:
	//{
	//     "brightness": 254,
	//     "color": {
	//         "hue": 42,
	//         "saturation": 42,
	//         "x": 42,
	//         "y": 42
	//     },
	//     "color_temp": 500,
	//     "color_temp_startup": 500,
	//     "effect": "blink",
	//     "power_on_behavior": "off",
	//     "state": "ON"
	//}
	//---------------

}

const testTokenUser = "testOwner"
const testtoken = `Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJqdGkiOiIwOGM0N2E4OC0yYzc5LTQyMGYtODEwNC02NWJkOWViYmU0MWUiLCJleHAiOjE1NDY1MDcyMzMsIm5iZiI6MCwiaWF0IjoxNTQ2NTA3MTczLCJpc3MiOiJodHRwOi8vbG9jYWxob3N0OjgwMDEvYXV0aC9yZWFsbXMvbWFzdGVyIiwiYXVkIjoiZnJvbnRlbmQiLCJzdWIiOiJ0ZXN0T3duZXIiLCJ0eXAiOiJCZWFyZXIiLCJhenAiOiJmcm9udGVuZCIsIm5vbmNlIjoiOTJjNDNjOTUtNzViMC00NmNmLTgwYWUtNDVkZDk3M2I0YjdmIiwiYXV0aF90aW1lIjoxNTQ2NTA3MDA5LCJzZXNzaW9uX3N0YXRlIjoiNWRmOTI4ZjQtMDhmMC00ZWI5LTliNjAtM2EwYWUyMmVmYzczIiwiYWNyIjoiMCIsImFsbG93ZWQtb3JpZ2lucyI6WyIqIl0sInJlYWxtX2FjY2VzcyI6eyJyb2xlcyI6WyJ1c2VyIl19LCJyZXNvdXJjZV9hY2Nlc3MiOnsibWFzdGVyLXJlYWxtIjp7InJvbGVzIjpbInZpZXctcmVhbG0iLCJ2aWV3LWlkZW50aXR5LXByb3ZpZGVycyIsIm1hbmFnZS1pZGVudGl0eS1wcm92aWRlcnMiLCJpbXBlcnNvbmF0aW9uIiwiY3JlYXRlLWNsaWVudCIsIm1hbmFnZS11c2VycyIsInF1ZXJ5LXJlYWxtcyIsInZpZXctYXV0aG9yaXphdGlvbiIsInF1ZXJ5LWNsaWVudHMiLCJxdWVyeS11c2VycyIsIm1hbmFnZS1ldmVudHMiLCJtYW5hZ2UtcmVhbG0iLCJ2aWV3LWV2ZW50cyIsInZpZXctdXNlcnMiLCJ2aWV3LWNsaWVudHMiLCJtYW5hZ2UtYXV0aG9yaXphdGlvbiIsIm1hbmFnZS1jbGllbnRzIiwicXVlcnktZ3JvdXBzIl19LCJhY2NvdW50Ijp7InJvbGVzIjpbIm1hbmFnZS1hY2NvdW50IiwibWFuYWdlLWFjY291bnQtbGlua3MiLCJ2aWV3LXByb2ZpbGUiXX19LCJyb2xlcyI6WyJ1c2VyIl19.ykpuOmlpzj75ecSI6cHbCATIeY4qpyut2hMc1a67Ycg`

const adminTokenUser = "admin"
const admintoken = `Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJqdGkiOiIwOGM0N2E4OC0yYzc5LTQyMGYtODEwNC02NWJkOWViYmU0MWUiLCJleHAiOjE1NDY1MDcyMzMsIm5iZiI6MCwiaWF0IjoxNTQ2NTA3MTczLCJpc3MiOiJodHRwOi8vbG9jYWxob3N0OjgwMDEvYXV0aC9yZWFsbXMvbWFzdGVyIiwiYXVkIjoiZnJvbnRlbmQiLCJzdWIiOiJhZG1pbiIsInR5cCI6IkJlYXJlciIsImF6cCI6ImZyb250ZW5kIiwibm9uY2UiOiI5MmM0M2M5NS03NWIwLTQ2Y2YtODBhZS00NWRkOTczYjRiN2YiLCJhdXRoX3RpbWUiOjE1NDY1MDcwMDksInNlc3Npb25fc3RhdGUiOiI1ZGY5MjhmNC0wOGYwLTRlYjktOWI2MC0zYTBhZTIyZWZjNzMiLCJhY3IiOiIwIiwiYWxsb3dlZC1vcmlnaW5zIjpbIioiXSwicmVhbG1fYWNjZXNzIjp7InJvbGVzIjpbInVzZXIiLCJhZG1pbiJdfSwicmVzb3VyY2VfYWNjZXNzIjp7Im1hc3Rlci1yZWFsbSI6eyJyb2xlcyI6WyJ2aWV3LXJlYWxtIiwidmlldy1pZGVudGl0eS1wcm92aWRlcnMiLCJtYW5hZ2UtaWRlbnRpdHktcHJvdmlkZXJzIiwiaW1wZXJzb25hdGlvbiIsImNyZWF0ZS1jbGllbnQiLCJtYW5hZ2UtdXNlcnMiLCJxdWVyeS1yZWFsbXMiLCJ2aWV3LWF1dGhvcml6YXRpb24iLCJxdWVyeS1jbGllbnRzIiwicXVlcnktdXNlcnMiLCJtYW5hZ2UtZXZlbnRzIiwibWFuYWdlLXJlYWxtIiwidmlldy1ldmVudHMiLCJ2aWV3LXVzZXJzIiwidmlldy1jbGllbnRzIiwibWFuYWdlLWF1dGhvcml6YXRpb24iLCJtYW5hZ2UtY2xpZW50cyIsInF1ZXJ5LWdyb3VwcyJdfSwiYWNjb3VudCI6eyJyb2xlcyI6WyJtYW5hZ2UtYWNjb3VudCIsIm1hbmFnZS1hY2NvdW50LWxpbmtzIiwidmlldy1wcm9maWxlIl19fSwicm9sZXMiOlsidXNlciIsImFkbWluIl19.ggcFFFEsjwdfSzEFzmZt_m6W4IiSQub2FRhZVfWttDI`

func TestDeviceTypeCreation(t *testing.T) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := configuration.Load("../../config.json")
	if err != nil {
		t.Error(err)
		return
	}

	config.MinCacheDuration = "100ms"
	config.MaxCacheDuration = "200ms"

	config.FallbackFile = filepath.Join(t.TempDir(), "fallback.json")

	config.DeviceManagerUrl, _, config.PermissionsSearchUrl, err = docker.DeviceManagerWithDependencies(ctx, wg)
	if err != nil {
		t.Error(err)
		return
	}

	//init protocol
	buf := &bytes.Buffer{}
	err = json.NewEncoder(buf).Encode(models.Protocol{
		Name:             "test",
		Handler:          "test",
		ProtocolSegments: []models.ProtocolSegment{{Name: "data"}},
	})
	if err != nil {
		t.Error(err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, config.DeviceManagerUrl+"/protocols", buf)
	if err != nil {
		t.Error(err)
		return
	}
	req.Header.Set("Authorization", admintoken)
	protocol, _, err := devicerepo.Do[models.Protocol](req)
	if err != nil {
		t.Error(err)
		return
	}

	config.CreateMissingDeviceTypesWithProtocol = protocol.Id
	config.CreateMissingDeviceTypesWithProtocolSegment = protocol.ProtocolSegments[0].Id

	mqttPort, _, err := docker.Mqtt(ctx, wg)
	if err != nil {
		t.Error(err)
		return
	}

	config.MgwMqttBroker = "tcp://localhost:" + mqttPort
	config.ZigbeeMqttBroker = "tcp://localhost:" + mqttPort

	c := Connector{
		config:         config,
		devicestate:    map[string]mgw.State{},
		devicestateMux: sync.Mutex{},
		zigbee:         &zigbee2mqtt.Client{},
	}
	c.mgw, err = mgw.New(ctx, wg, config, func() {})
	if err != nil {
		t.Error(err)
		return
	}
	c.devicerepo, err = devicerepo.New(config, mocks.Auth(testtoken))
	if err != nil {
		t.Error(err)
		return
	}

	options := paho.NewClientOptions().
		SetAutoReconnect(true).
		SetCleanSession(true).
		SetClientID("test-watcher").
		AddBroker(config.ZigbeeMqttBroker).
		SetWriteTimeout(10 * time.Second).
		SetOrderMatters(false).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			log.Println("connection to test watcher broker lost")
		}).
		SetOnConnectHandler(func(_ paho.Client) {
			log.Println("connected to test watcher broker")
		})

	testwatcher := paho.NewClient(options)
	if token := testwatcher.Connect(); token.Wait() && token.Error() != nil {
		t.Error(token.Error())
		return
	}

	mqttMessages := map[string][]string{}
	testwatchermux := sync.Mutex{}
	testwatcher.Subscribe("#", 2, func(client paho.Client, message paho.Message) {
		testwatchermux.Lock()
		defer testwatchermux.Unlock()
		mqttMessages[message.Topic()] = append(mqttMessages[message.Topic()], string(message.Payload()))
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		testwatcher.Disconnect(200)
	}()

	getMessages := func() (result map[string][]string) {
		testwatchermux.Lock()
		defer testwatchermux.Unlock()
		temp, _ := json.Marshal(mqttMessages)
		json.Unmarshal(temp, &result)
		return result
	}

	c.handleDeviceInfoUpdate(resources.DeviceInfoExample[1])

	time.Sleep(2 * time.Second)

	dtId := ""
	t.Run("check device repo", func(t *testing.T) {
		var usedFallback bool
		dtId, usedFallback, err = c.devicerepo.FindDeviceTypeId(resources.DeviceInfoExample[1])
		if err != nil {
			t.Error(err)
			return
		}
		if usedFallback {
			t.Error(usedFallback)
			return
		}
		req, err := http.NewRequest(http.MethodGet, config.DeviceManagerUrl+"/device-types/"+url.PathEscape(dtId), buf)
		if err != nil {
			t.Error(err)
			return
		}
		req.Header.Set("Authorization", testtoken)
		dt, _, err := devicerepo.Do[models.DeviceType](req)
		if err != nil {
			t.Error(err)
			return
		}
		prepareForTestCompare(&dt, t)
		expected := models.DeviceType{
			Name:        "Philips 9290012573A",
			Description: "",
			Services: []models.Service{
				{
					LocalId:     "get",
					Name:        "get",
					Interaction: "event+request",
					ProtocolId:  protocol.Id,
					Outputs: []models.Content{
						{
							ContentVariable: models.ContentVariable{
								Name:   "value",
								IsVoid: false,
								Type:   "https://schema.org/StructuredValue",
								SubContentVariables: []models.ContentVariable{
									{
										Name:   "state",
										IsVoid: false,
										Type:   "https://schema.org/Text",
									},
									{
										Name:   "brightness",
										IsVoid: false,
										Type:   "https://schema.org/Float",
									},
									{
										Name:   "power_on_behavior",
										IsVoid: false,
										Type:   "https://schema.org/Text",
									},
									{
										Name:   "linkquality",
										IsVoid: false,
										Type:   "https://schema.org/Float",
									},
									{
										Name:   "color_temp",
										IsVoid: false,
										Type:   "https://schema.org/Float",
									},
									{
										Name:   "color_temp_startup",
										IsVoid: false,
										Type:   "https://schema.org/Float",
									},
									{
										Name:   "color",
										IsVoid: false,
										Type:   "https://schema.org/StructuredValue",
										SubContentVariables: []models.ContentVariable{
											{
												Name:   "x",
												IsVoid: false,
												Type:   "https://schema.org/Float",
											},
											{
												Name:   "y",
												IsVoid: false,
												Type:   "https://schema.org/Float",
											},
											{
												Name:   "hue",
												IsVoid: false,
												Type:   "https://schema.org/Float",
											},
											{
												Name:   "saturation",
												IsVoid: false,
												Type:   "https://schema.org/Float",
											},
										},
									},
								},
							},
							Serialization:     "json",
							ProtocolSegmentId: protocol.ProtocolSegments[0].Id,
						},
					},
				},
				{
					LocalId:     "set",
					Name:        "set",
					Interaction: "request",
					ProtocolId:  protocol.Id,
					Inputs: []models.Content{
						{
							ContentVariable: models.ContentVariable{
								Name:   "value",
								IsVoid: false,
								Type:   "https://schema.org/StructuredValue",
								SubContentVariables: []models.ContentVariable{
									{
										Name:   "brightness",
										IsVoid: false,
										Type:   "https://schema.org/Float",
									},
									{
										Name:   "color_temp",
										IsVoid: false,
										Type:   "https://schema.org/Float",
									},
									{
										Name:   "color_temp_startup",
										IsVoid: false,
										Type:   "https://schema.org/Float",
									},
									{
										Name:   "color",
										IsVoid: false,
										Type:   "https://schema.org/StructuredValue",
										SubContentVariables: []models.ContentVariable{
											{
												Name:   "hue",
												IsVoid: false,
												Type:   "https://schema.org/Float",
											},
											{
												Name:   "saturation",
												IsVoid: false,
												Type:   "https://schema.org/Float",
											},
											{
												Name:   "x",
												IsVoid: false,
												Type:   "https://schema.org/Float",
											},
											{
												Name:   "y",
												IsVoid: false,
												Type:   "https://schema.org/Float",
											},
										},
									},
									{
										Name:   "effect",
										IsVoid: false,
										Type:   "https://schema.org/Text",
									},
									{
										Name:   "power_on_behavior",
										IsVoid: false,
										Type:   "https://schema.org/Text",
									},
									{
										Name:   "state",
										IsVoid: false,
										Type:   "https://schema.org/Text",
									},
								},
							},
							Serialization:     "json",
							ProtocolSegmentId: protocol.ProtocolSegments[0].Id},
					},
				},
			},
			DeviceClassId: "urn:infai:ses:device-class:ff64280a-58e6-4cf9-9a44-e70d3831a79d",
			Attributes: []models.Attribute{
				{Key: "senergy/zigbee-dc", Value: "true", Origin: ""},
				{Key: "senergy/zigbee-vendor", Value: "Philips", Origin: ""},
				{Key: "senergy/zigbee-model", Value: "9290012573A", Origin: ""},
			},
		}
		prepareForTestCompare(&expected, nil)

		if !reflect.DeepEqual(dt, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, dt)
			actualJson, _ := json.Marshal(dt)
			expectedJson, _ := json.Marshal(expected)
			t.Log(string(actualJson))
			t.Log(string(expectedJson))
		}
	})

	t.Run("check mgw messages", func(t *testing.T) {
		expected := map[string][]string{
			"device-manager/device/mgw-zigbee-dc": []string{
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"" + dtId + "\"}}",
			},
		}
		actualMessages := getMessages()
		if !reflect.DeepEqual(actualMessages, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actualMessages)
			actualJson, _ := json.Marshal(actualMessages)
			expectedJson, _ := json.Marshal(expected)
			t.Log(string(actualJson))
			t.Log(string(expectedJson))
		}
	})

}

func prepareForTestCompare(e interface{}, t *testing.T) {
	switch v := e.(type) {
	case *models.DeviceType:
		if v.Id == "" && t != nil {
			t.Error(v)
			return
		}
		v.Id = ""
		for i, sub := range v.Services {
			prepareForTestCompare(&sub, t)
			v.Services[i] = sub
		}
		sort.Slice(v.Services, func(i, j int) bool {
			return v.Services[i].Name < v.Services[j].Name
		})
	case *models.Service:
		if v.Id == "" && t != nil {
			t.Error(v)
			return
		}
		v.Id = ""
		for i, sub := range v.Inputs {
			prepareForTestCompare(&sub, t)
			v.Inputs[i] = sub
		}
		for i, sub := range v.Outputs {
			prepareForTestCompare(&sub, t)
			v.Outputs[i] = sub
		}
	case *models.Content:
		if v.Id == "" && t != nil {
			t.Error(v)
			return
		}
		v.Id = ""
		prepareForTestCompare(&v.ContentVariable, t)
	case *models.ContentVariable:
		if v.Id == "" && t != nil {
			t.Error(v)
			return
		}
		v.Id = ""
		for i, sub := range v.SubContentVariables {
			prepareForTestCompare(&sub, t)
			v.SubContentVariables[i] = sub
		}
		sort.Slice(v.SubContentVariables, func(i, j int) bool {
			return v.SubContentVariables[i].Name < v.SubContentVariables[j].Name
		})
	}
}
