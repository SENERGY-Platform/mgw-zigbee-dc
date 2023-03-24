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

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/connector"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/devicerepo"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/docker"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/mocks"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/resources"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/zigbee2mqtt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"log"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestIntegration(t *testing.T) {
	fallbackFile := filepath.Join(t.TempDir(), "fallback.json")
	t.Run("initial", func(t *testing.T) {
		testIntegrationWithWorkingPermSearch(t, fallbackFile)
	})
	t.Run("fallback", func(t *testing.T) {
		testIntegrationWithFallbackUse(t, fallbackFile)
	})
}

func testIntegrationWithWorkingPermSearch(t *testing.T, fallbackFile string) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := configuration.Load("../../config.json")
	if err != nil {
		t.Error(err)
		return
	}

	config.MinCacheDuration = "200ms"
	config.MaxCacheDuration = "1s"
	config.FallbackFile = fallbackFile

	mqttPort, _, err := docker.Mqtt(ctx, wg)
	if err != nil {
		t.Error(err)
		return
	}

	config.MgwMqttBroker = "tcp://localhost:" + mqttPort
	config.ZigbeeMqttBroker = "tcp://localhost:" + mqttPort

	var permCtrl *mocks.PermissionsSearch
	config.PermissionsSearchUrl, permCtrl, err = mocks.StartPermissionsSearch(ctx, wg)
	permCtrl.SetResp([]model.DeviceType{}, nil, 200)

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

	zigbeemock, err := mocks.NewZigbeeMock(ctx, wg, config)
	if err != nil {
		t.Error(err)
		return
	}

	deviceId := "Hue Ingo"
	zigbeemock.SendDeviceInfos(resources.DeviceInfoExample)
	zigbeemock.TriggerEvent(deviceId, map[string]interface{}{
		"brightness": 254,
		"color": map[string]interface{}{
			"hue":        42,
			"saturation": 42,
			"x":          42,
			"y":          42,
		},
		"color_temp":         500,
		"color_temp_startup": 500,
		"linkquality":        255,
		"power_on_behavior":  "off",
		"state":              "ON",
	})

	time.Sleep(200 * time.Millisecond)

	deviceRepo, err := devicerepo.New(config, mocks.Auth("testtoken"))
	if err != nil {
		t.Error(err)
		return
	}
	_, err = connector.StartWithDependencies(ctx, wg, config, connector.MgwFactoryCast(mgw.New), connector.ZigbeeFactoryCast(zigbee2mqtt.New), deviceRepo)
	if err != nil {
		t.Error(err)
		return
	}

	getMessages := func() (result map[string][]string) {
		testwatchermux.Lock()
		defer testwatchermux.Unlock()
		temp, _ := json.Marshal(mqttMessages)
		json.Unmarshal(temp, &result)
		return result
	}

	const expectedDtErrInfo = "mgw-zigbee-dc: missing zigbee device-type, please provide a device-type with:\nref: https://www.zigbee2mqtt.io/devices/9290012573A.html\nattributes:\n    - senergy/zigbee-dc = true\n    - senergy/zigbee-vendor = Philips\n    - senergy/zigbee-model = 9290012573A\nservices:\n---------------\nlocal-id: get\nprotocol: standard-connector\nexample output data:\n{\n    \"brightness\": 254,\n    \"color\": {\n        \"hue\": 42,\n        \"saturation\": 42,\n        \"x\": 42,\n        \"y\": 42\n    },\n    \"color_temp\": 500,\n    \"color_temp_startup\": 500,\n    \"linkquality\": 255,\n    \"power_on_behavior\": \"off\",\n    \"state\": \"ON\"\n}\n---------------\nlocal-id: set\nprotocol: standard-connector\nexample input data:\n{\n    \"brightness\": 254,\n    \"color\": {\n        \"hue\": 42,\n        \"saturation\": 42,\n        \"x\": 42,\n        \"y\": 42\n    },\n    \"color_temp\": 500,\n    \"color_temp_startup\": 500,\n    \"effect\": \"blink\",\n    \"power_on_behavior\": \"off\",\n    \"state\": \"ON\"\n}\n---------------\n"

	temp, err := json.Marshal(resources.DeviceInfoExample)
	if err != nil {
		t.Error(err)
		return
	}
	expectedDeviceInfoList := string(temp)

	t.Run("after startup without existing device-type", func(t *testing.T) {
		time.Sleep(2 * time.Second)
		expected := map[string][]string{
			"error/client": {
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption again :/ (error in mqtt broker or client?)
				expectedDtErrInfo, //initial NotifyRefresh() call by mgw lib
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
			}}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
		}
	})

	t.Run("reaction to notify request without device-types", func(t *testing.T) {
		token := testwatcher.Publish("device-manager/refresh", 2, false, "")
		token.Wait()
		err = token.Error()
		if err != nil {
			t.Error(err)
			return
		}
		time.Sleep(1 * time.Second)

		expected := map[string][]string{
			"error/client": {
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption again :/ (error in mqtt broker or client?)
				expectedDtErrInfo, //initial NotifyRefresh() call by mgw lib
				expectedDtErrInfo, //from device-manager/refresh
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
			},
		}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
		}
	})

	t.Run("new device-types, refresh after config.MaxCacheDuration wait", func(t *testing.T) {
		permCtrl.SetResp([]model.DeviceType{{
			Id:          "dt-id",
			Name:        "dt-name",
			Description: "dt-desc",
			Attributes: []model.Attribute{
				{Key: devicerepo.AttributeUsedForZigbee, Value: "true"},
				{Key: devicerepo.AttributeZigbeeVendor, Value: resources.DeviceInfoExample[1].Definition.Vendor},
				{Key: devicerepo.AttributeZigbeeModel, Value: resources.DeviceInfoExample[1].Definition.Model},
			},
		}}, nil, 200)
		time.Sleep(2 * time.Second)

		token := testwatcher.Publish("device-manager/refresh", 2, false, "")
		token.Wait()
		err = token.Error()
		if err != nil {
			t.Error(err)
			return
		}
		time.Sleep(1 * time.Second)

		expected := map[string][]string{
			"error/client": {
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption again :/ (error in mqtt broker or client?)
				expectedDtErrInfo, //initial NotifyRefresh() call by mgw lib
				expectedDtErrInfo, //from device-manager/refresh
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
				"",
			},
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
		}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
		}
	})

	t.Run("zigbee mock devices signal", func(t *testing.T) {
		zigbeemock.SendDeviceInfos(resources.DeviceInfoExample)
		time.Sleep(1 * time.Second)
		expected := map[string][]string{
			"error/client": {
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption again :/ (error in mqtt broker or client?)
				expectedDtErrInfo, //initial NotifyRefresh() call by mgw lib
				expectedDtErrInfo, //from device-manager/refresh
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
				"",
			},
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
		}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
		}
	})

	t.Run("event", func(t *testing.T) {
		zigbeemock.TriggerEvent(deviceId, map[string]interface{}{
			"state": "OFF",
		})
		time.Sleep(time.Second)
		expected := map[string][]string{
			"error/client": {
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption again :/ (error in mqtt broker or client?)
				expectedDtErrInfo, //initial NotifyRefresh() call by mgw lib
				expectedDtErrInfo, //from device-manager/refresh
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",  //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",  //zigbee2mqtt/Hue Ingo/get response
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}", //event
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
				"",
			},
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
			"event/zigbee:0x00178801020a70e7/get": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}",
			},
		}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
		}
	})

	t.Run("set", func(t *testing.T) {
		token := testwatcher.Publish("command/zigbee:0x00178801020a70e7/set", 2, false, `{"command_id": "1", "data": "{\"brightness\": 100}"}`)
		token.Wait()
		err = token.Error()
		if err != nil {
			t.Error(err)
			return
		}
		time.Sleep(2 * time.Second)
		expected := map[string][]string{
			"error/client": {
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption again :/ (error in mqtt broker or client?)
				expectedDtErrInfo, //initial NotifyRefresh() call by mgw lib
				expectedDtErrInfo, //from device-manager/refresh
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",  //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",  //zigbee2mqtt/Hue Ingo/get response
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}", //event
				"{\"brightness\":100,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}", //set
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
				"",
			},
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
			"event/zigbee:0x00178801020a70e7/get": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}", //event
				"{\"brightness\":100,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}", //set
			},
			"command/zigbee:0x00178801020a70e7/set": {
				"{\"command_id\": \"1\", \"data\": \"{\\\"brightness\\\": 100}\"}",
			},
			"zigbee2mqtt/Hue Ingo/set": {
				"{\"brightness\": 100}",
			},
			"response/zigbee:0x00178801020a70e7/set": {"{\"command_id\":\"1\",\"data\":\"\"}"},
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

	t.Run("get", func(t *testing.T) {
		token := testwatcher.Publish("command/zigbee:0x00178801020a70e7/get", 2, false, `{"command_id": "2", "data": ""}`)
		token.Wait()
		err = token.Error()
		if err != nil {
			t.Error(err)
			return
		}
		time.Sleep(2 * time.Second)
		expected := map[string][]string{
			"error/client": {
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption
				expectedDtErrInfo, //retained zigbee2mqtt/bridge/devices consumption again :/ (error in mqtt broker or client?)
				expectedDtErrInfo, //initial NotifyRefresh() call by mgw lib
				expectedDtErrInfo, //from device-manager/refresh
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",  //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",  //zigbee2mqtt/Hue Ingo/get response
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}", //event
				"{\"brightness\":100,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}", //set
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
				"",
			},
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
			"event/zigbee:0x00178801020a70e7/get": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}", //event
				"{\"brightness\":100,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"OFF\"}", //set
			},
			"command/zigbee:0x00178801020a70e7/set": {
				"{\"command_id\": \"1\", \"data\": \"{\\\"brightness\\\": 100}\"}",
			},
			"command/zigbee:0x00178801020a70e7/get": {
				"{\"command_id\": \"2\", \"data\": \"\"}",
			},
			"zigbee2mqtt/Hue Ingo/set": {
				"{\"brightness\": 100}",
			},
			"response/zigbee:0x00178801020a70e7/set": {
				"{\"command_id\":\"1\",\"data\":\"\"}",
			},
			"response/zigbee:0x00178801020a70e7/get": {
				"{\"command_id\":\"2\",\"data\":\"{\\\"brightness\\\":100,\\\"color\\\":{\\\"hue\\\":42,\\\"saturation\\\":42,\\\"x\\\":42,\\\"y\\\":42},\\\"color_temp\\\":500,\\\"color_temp_startup\\\":500,\\\"linkquality\\\":255,\\\"power_on_behavior\\\":\\\"off\\\",\\\"state\\\":\\\"OFF\\\"}\"}",
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

func testIntegrationWithFallbackUse(t *testing.T, fallbackFile string) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := configuration.Load("../../config.json")
	if err != nil {
		t.Error(err)
		return
	}

	config.MinCacheDuration = "200ms"
	config.MaxCacheDuration = "1s"
	config.FallbackFile = fallbackFile

	mqttPort, _, err := docker.Mqtt(ctx, wg)
	if err != nil {
		t.Error(err)
		return
	}

	config.MgwMqttBroker = "tcp://localhost:" + mqttPort
	config.ZigbeeMqttBroker = "tcp://localhost:" + mqttPort

	var permCtrl *mocks.PermissionsSearch
	config.PermissionsSearchUrl, permCtrl, err = mocks.StartPermissionsSearch(ctx, wg)
	permCtrl.SetResp(nil, errors.New("test error"), 500)

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

	zigbeemock, err := mocks.NewZigbeeMock(ctx, wg, config)
	if err != nil {
		t.Error(err)
		return
	}

	deviceId := "Hue Ingo"
	zigbeemock.SendDeviceInfos(resources.DeviceInfoExample)
	zigbeemock.TriggerEvent(deviceId, map[string]interface{}{
		"brightness": 254,
		"color": map[string]interface{}{
			"hue":        42,
			"saturation": 42,
			"x":          42,
			"y":          42,
		},
		"color_temp":         500,
		"color_temp_startup": 500,
		"linkquality":        255,
		"power_on_behavior":  "off",
		"state":              "ON",
	})

	time.Sleep(200 * time.Millisecond)

	deviceRepo, err := devicerepo.New(config, mocks.Auth("testtoken"))
	if err != nil {
		t.Error(err)
		return
	}
	_, err = connector.StartWithDependencies(ctx, wg, config, connector.MgwFactoryCast(mgw.New), connector.ZigbeeFactoryCast(zigbee2mqtt.New), deviceRepo)
	if err != nil {
		t.Error(err)
		return
	}

	getMessages := func() (result map[string][]string) {
		testwatchermux.Lock()
		defer testwatchermux.Unlock()
		temp, _ := json.Marshal(mqttMessages)
		json.Unmarshal(temp, &result)
		return result
	}

	const expectedDtErrInfo = "mgw-zigbee-dc: missing zigbee device-type, please provide a device-type with:\nref: https://www.zigbee2mqtt.io/devices/9290012573A.html\nattributes:\n    - senergy/zigbee-dc = true\n    - senergy/zigbee-vendor = Philips\n    - senergy/zigbee-model = 9290012573A\nservices:\n---------------\nlocal-id: get\nprotocol: standard-connector\nexample output data:\n{\n    \"brightness\": 254,\n    \"color\": {\n        \"hue\": 42,\n        \"saturation\": 42,\n        \"x\": 42,\n        \"y\": 42\n    },\n    \"color_temp\": 500,\n    \"color_temp_startup\": 500,\n    \"linkquality\": 255,\n    \"power_on_behavior\": \"off\",\n    \"state\": \"ON\"\n}\n---------------\nlocal-id: set\nprotocol: standard-connector\nexample input data:\n{\n    \"brightness\": 254,\n    \"color\": {\n        \"hue\": 42,\n        \"saturation\": 42,\n        \"x\": 42,\n        \"y\": 42\n    },\n    \"color_temp\": 500,\n    \"color_temp_startup\": 500,\n    \"effect\": \"blink\",\n    \"power_on_behavior\": \"off\",\n    \"state\": \"ON\"\n}\n---------------\n"

	temp, err := json.Marshal(resources.DeviceInfoExample)
	if err != nil {
		t.Error(err)
		return
	}
	expectedDeviceInfoList := string(temp)

	t.Run("after startup", func(t *testing.T) {
		time.Sleep(2 * time.Second)
		expected := map[string][]string{
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"event/zigbee:0x00178801020a70e7/get": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
			},
		}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
			actualJson, _ := json.Marshal(actual)
			expectedJson, _ := json.Marshal(expected)
			t.Log("\n", string(actualJson), "\n", string(expectedJson))
		}
	})

	t.Run("refresh notify", func(t *testing.T) {
		token := testwatcher.Publish("device-manager/refresh", 2, false, "")
		token.Wait()
		err = token.Error()
		if err != nil {
			t.Error(err)
			return
		}
		time.Sleep(1 * time.Second)

		expected := map[string][]string{
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"event/zigbee:0x00178801020a70e7/get": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
			},
		}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
			actualJson, _ := json.Marshal(actual)
			expectedJson, _ := json.Marshal(expected)
			t.Log("\n", string(actualJson), "\n", string(expectedJson))
		}
	})

	t.Run("zigbee mock devices signal", func(t *testing.T) {
		zigbeemock.SendDeviceInfos(resources.DeviceInfoExample)
		time.Sleep(1 * time.Second)
		expected := map[string][]string{
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"event/zigbee:0x00178801020a70e7/get": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
			},
		}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
			actualJson, _ := json.Marshal(actual)
			expectedJson, _ := json.Marshal(expected)
			t.Log("\n", string(actualJson), "\n", string(expectedJson))
		}
	})

	t.Run("set", func(t *testing.T) {
		token := testwatcher.Publish("command/zigbee:0x00178801020a70e7/set", 2, false, `{"command_id": "1", "data": "{\"brightness\": 100}"}`)
		token.Wait()
		err = token.Error()
		if err != nil {
			t.Error(err)
			return
		}
		time.Sleep(2 * time.Second)

		expected := map[string][]string{
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
				"{\"brightness\":100,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"event/zigbee:0x00178801020a70e7/get": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",
				"{\"brightness\":100,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
			},
			"command/zigbee:0x00178801020a70e7/set": {
				"{\"command_id\": \"1\", \"data\": \"{\\\"brightness\\\": 100}\"}",
			},
			"zigbee2mqtt/Hue Ingo/set": {
				"{\"brightness\": 100}",
			},
			"response/zigbee:0x00178801020a70e7/set": {"{\"command_id\":\"1\",\"data\":\"\"}"},
		}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
			actualJson, _ := json.Marshal(actual)
			expectedJson, _ := json.Marshal(expected)
			t.Log("\n", string(actualJson), "\n", string(expectedJson))
		}
	})

	t.Run("get", func(t *testing.T) {
		token := testwatcher.Publish("command/zigbee:0x00178801020a70e7/get", 2, false, `{"command_id": "2", "data": ""}`)
		token.Wait()
		err = token.Error()
		if err != nil {
			t.Error(err)
			return
		}
		time.Sleep(2 * time.Second)

		expected := map[string][]string{
			"device-manager/device/mgw-zigbee-dc": {
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
				"{\"method\":\"set\",\"device_id\":\"zigbee:0x00178801020a70e7\",\"data\":{\"name\":\"Hue Ingo\",\"state\":\"online\",\"device_type\":\"dt-id\"}}",
			},
			"zigbee2mqtt/Hue Ingo": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //initial event send by mock, not consumed by connector
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
				"{\"brightness\":100,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}", //zigbee2mqtt/Hue Ingo/get response
			},
			"zigbee2mqtt/Hue Ingo/get": {
				"{\"brightness\":null,\"color\":null,\"color_temp\":null,\"color_temp_startup\":null,\"power_on_behavior\":null,\"state\":null}", //triggered by retained zigbee2mqtt/bridge/devices message consumption
			},
			"event/zigbee:0x00178801020a70e7/get": {
				"{\"brightness\":254,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",
				"{\"brightness\":100,\"color\":{\"hue\":42,\"saturation\":42,\"x\":42,\"y\":42},\"color_temp\":500,\"color_temp_startup\":500,\"linkquality\":255,\"power_on_behavior\":\"off\",\"state\":\"ON\"}",
			},
			"zigbee2mqtt/bridge/devices": {
				expectedDeviceInfoList,
				expectedDeviceInfoList,
			},
			"device-manager/refresh": {
				"",
			},
			"command/zigbee:0x00178801020a70e7/set": {
				"{\"command_id\": \"1\", \"data\": \"{\\\"brightness\\\": 100}\"}",
			},
			"zigbee2mqtt/Hue Ingo/set": {
				"{\"brightness\": 100}",
			},
			"command/zigbee:0x00178801020a70e7/get": {
				"{\"command_id\": \"2\", \"data\": \"\"}",
			},
			"response/zigbee:0x00178801020a70e7/set": {"{\"command_id\":\"1\",\"data\":\"\"}"},
			"response/zigbee:0x00178801020a70e7/get": {
				"{\"command_id\":\"2\",\"data\":\"{\\\"brightness\\\":100,\\\"color\\\":{\\\"hue\\\":42,\\\"saturation\\\":42,\\\"x\\\":42,\\\"y\\\":42},\\\"color_temp\\\":500,\\\"color_temp_startup\\\":500,\\\"linkquality\\\":255,\\\"power_on_behavior\\\":\\\"off\\\",\\\"state\\\":\\\"ON\\\"}\"}",
			},
		}

		actual := getMessages()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\n%#v\n%#v\n", expected, actual)
			actualJson, _ := json.Marshal(actual)
			expectedJson, _ := json.Marshal(expected)
			t.Log("\n", string(actualJson), "\n", string(expectedJson))
		}
	})
}
