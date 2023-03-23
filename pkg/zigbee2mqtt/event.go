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
	"encoding/json"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	paho "github.com/eclipse/paho.mqtt.golang"
	"log"
	"runtime/debug"
	"strings"
)

func (this *Client) startEventListener() error {
	token := this.mqtt.Subscribe(this.getEventTopic(), byte(this.config.ZigbeeQos), this.eventHandler)
	token.Wait()
	return token.Error()
}

func (this *Client) getEventTopic() string {
	return this.config.ZigbeeMqttTopicPrefix + "#"
}

func (this *Client) eventHandler(client paho.Client, message paho.Message) {
	topic := message.Topic()
	if strings.HasPrefix(topic, this.config.ZigbeeMqttTopicPrefix+"bridge/") {
		return
	}
	deviceName := strings.TrimPrefix(topic, this.config.ZigbeeMqttTopicPrefix)
	device, err := this.register.GetByFriendlyName(nil, deviceName)
	if err == model.ErrNotFound {
		return
	}
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
		return
	}
	this.valueMux.Lock()
	defer this.valueMux.Unlock()
	this.values[device.IeeeAddress] = message.Payload()
	this.connector.Event(device, message.Payload())
}

func (this *Client) RefreshEventValues() error {
	devices, err := this.register.List(nil)
	if err != nil {
		return err
	}
	for _, device := range devices {
		if !device.Disabled && device.Supported && device.InterviewCompleted {
			err = this.RefreshDeviceEventValues(device)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (this *Client) RefreshDeviceEventValues(device model.ZigbeeDeviceInfo) error {
	pl, err := json.Marshal(getZigbeeGetMessage(device))
	if err != nil {
		return err
	}
	return this.send(this.config.ZigbeeMqttTopicPrefix+device.FriendlyName+"/get", 2, false, pl)
}

func getZigbeeGetMessage(device model.ZigbeeDeviceInfo) (req map[string]interface{}) {
	req = map[string]interface{}{}
	todo := []model.ZigbeeDeviceInfoFeature{}
	todo = append(todo, device.Definition.Exposes...)
	for i := 0; i < len(todo); i++ {
		field := todo[i]
		if field.Property != "" {
			if field.Access == 0 || checkZigbeeAccess(field.Access, ZigbeeGetAccess) {
				req[field.Property] = nil
			}
		} else if len(field.Features) > 0 {
			todo = append(todo, field.Features...)
		}
	}
	return
}
