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
	"context"
	"encoding/json"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	paho "github.com/eclipse/paho.mqtt.golang"
	"log"
)

func (this *Client) startDeviceInfoListener() error {
	token := this.mqtt.Subscribe(this.getDeviceInfoTopic(), byte(this.config.ZigbeeQos), this.deviceInfoHandler)
	token.Wait()
	return token.Error()
}

func (this *Client) getDeviceInfoTopic() string {
	return this.config.ZigbeeMqttTopicPrefix + "bridge/devices"
}

func (this *Client) deviceInfoHandler(client paho.Client, message paho.Message) {
	devices := []model.ZigbeeDeviceInfo{}
	err := json.Unmarshal(message.Payload(), &devices)
	if err != nil {
		log.Println("ERROR: unable to handle device info message", err)
		return
	}
	this.register.Set(devices)
	if len(this.values) == 0 {
		err = this.RefreshEventValues()
		if err != nil {
			log.Println("ERROR: unable to refresh event values", err)
			return
		}
	}
	this.connector.DeviceUpdate(devices)
}

// GetDeviceList returns a list of model.ZigbeeDeviceInfo
// ctx may be nil (defaults to context with 1 minute timeout)
// if no device is found err == model.ErrNotFound
// if the ctx is done before the register received its first Set() call, GetDeviceList() returns the ctx error
// except if the ctx error is context.Canceled, then no error is returned
func (this *Client) GetDeviceList(ctx context.Context) ([]model.ZigbeeDeviceInfo, error) {
	return this.register.List(ctx)
}
