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

package zigbee2mqtt

import (
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/resources"
	"reflect"
	"testing"
)

func TestGetZigbeeGetMessage(t *testing.T) {
	msg := getZigbeeGetMessage(resources.DeviceInfoExample[1])
	expected := map[string]interface{}{
		"brightness":         nil,
		"color":              nil,
		"color_temp":         nil,
		"color_temp_startup": nil,
		"power_on_behavior":  nil,
		"state":              nil,
	}

	if !reflect.DeepEqual(expected, msg) {
		t.Errorf("\n%#v\n%#v\n", expected, msg)
	}
}

type fakeMessage struct {
	topic    string
	payload  []byte
	retained bool
}

func (m fakeMessage) Duplicate() bool   { return false }
func (m fakeMessage) Qos() byte         { return 2 }
func (m fakeMessage) Retained() bool    { return m.retained }
func (m fakeMessage) Topic() string     { return m.topic }
func (m fakeMessage) MessageID() uint16 { return 0 }
func (m fakeMessage) Payload() []byte   { return m.payload }
func (m fakeMessage) Ack()              {}

type recordingConnector struct {
	events []string
	states []string
}

func (c *recordingConnector) Event(device model.ZigbeeDeviceInfo, payload []byte) {
	c.events = append(c.events, device.IeeeAddress+" "+string(payload))
}
func (c *recordingConnector) StateObserved(ieee string, payload []byte) {
	c.states = append(c.states, ieee+" "+string(payload))
}
func (c *recordingConnector) DeviceInfoUpdate([]model.ZigbeeDeviceInfo) {}

func TestEventHandlerStateObserved(t *testing.T) {
	device := resources.DeviceInfoExample[1]
	conn := &recordingConnector{}
	client := &Client{
		config:    configuration.Config{ZigbeeMqttTopicPrefix: "zigbee2mqtt/"},
		register:  NewDeviceRegister(),
		values:    map[string][]byte{},
		connector: conn,
	}
	client.register.Set(resources.DeviceInfoExample)
	topic := "zigbee2mqtt/" + device.FriendlyName
	state := `{"state":"ON"}`

	client.eventHandler(nil, fakeMessage{topic: topic, payload: []byte(state)})
	client.eventHandler(nil, fakeMessage{topic: topic, payload: []byte(state), retained: true})
	client.eventHandler(nil, fakeMessage{topic: "zigbee2mqtt/unknown device", payload: []byte(state)})
	client.eventHandler(nil, fakeMessage{topic: "zigbee2mqtt/bridge/state", payload: []byte(state)})

	want := device.IeeeAddress + " " + state
	if !reflect.DeepEqual(conn.states, []string{want}) {
		t.Errorf("states (retained must be ignored): %#v", conn.states)
	}
	if !reflect.DeepEqual(conn.events, []string{want, want}) {
		t.Errorf("events (forwarding unchanged, retained included): %#v", conn.events)
	}
}
