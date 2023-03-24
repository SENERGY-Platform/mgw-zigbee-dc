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
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
)

func (this *Client) SetByIeee(ieeeAddress string, pl []byte) error {
	device, err := this.register.GetByIeee(nil, ieeeAddress)
	if err != nil {
		return err
	}
	return this.send(this.config.ZigbeeMqttTopicPrefix+device.FriendlyName+"/set", byte(this.config.ZigbeeQos), false, pl)
}

func (this *Client) SetByFriendlyName(name string, pl []byte) error {
	device, err := this.register.GetByFriendlyName(nil, name)
	if err != nil {
		return err
	}
	return this.send(this.config.ZigbeeMqttTopicPrefix+device.FriendlyName+"/set", byte(this.config.ZigbeeQos), false, pl)
}

func (this *Client) GetByIeee(ieeeAddress string) ([]byte, error) {
	device, err := this.register.GetByIeee(nil, ieeeAddress)
	if err != nil {
		return nil, err
	}
	return this.Get(device)
}

func (this *Client) GetByFriendlyName(name string) ([]byte, error) {
	device, err := this.register.GetByFriendlyName(nil, name)
	if err != nil {
		return nil, err
	}
	return this.Get(device)
}

func (this *Client) Get(device model.ZigbeeDeviceInfo) ([]byte, error) {
	this.valueMux.Lock()
	defer this.valueMux.Unlock()
	value, ok := this.values[device.IeeeAddress]
	if ok {
		return value, nil
	}
	return nil, model.ErrNoValueFound
}

func (this *Client) send(topic string, qos byte, retained bool, payload interface{}) error {
	if this.mqtt == nil {
		return model.ErrNotReady
	}
	token := this.mqtt.Publish(topic, qos, retained, payload)
	token.Wait()
	return token.Error()
}
