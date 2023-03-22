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
	"context"
	"errors"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"log"
	"runtime/debug"
	"strings"
	"sync"
)

func (this *Connector) startDeviceHandling(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("start device info handling")
		for {
			select {
			case <-ctx.Done():
				log.Println("stop device info handling")
				return
			case devices := <-this.deviceupdatebuffer:
				for _, device := range devices {
					this.handleDeviceInfoUpdate(device)
				}
			}
		}
	}()
	return nil
}

func (this *Connector) handleDeviceInfoUpdate(device model.ZigbeeDeviceInfo) {
	if device.Supported && device.InterviewCompleted {
		deviceId := this.getDeviceId(device)
		deviceName := this.getDeviceName(device)
		deviceTypeId, err := this.getDeviceTypeId(device)
		if errors.Is(err, model.NoMatchingDeviceTypeFound) {
			log.Println("WARNING: unable to find matching device type", err)
			this.mgw.SendClientError(err.Error())
			return
		}
		if err != nil {
			log.Println("ERROR:", err)
			debug.PrintStack()
			return
		}
		deviceState := this.getDeviceState(device)
		err = this.mgw.SetDevice(deviceId, mgw.DeviceInfo{
			Name:       deviceName,
			State:      deviceState,
			DeviceType: deviceTypeId,
		})
		if err != nil {
			log.Println("ERROR:", err)
			debug.PrintStack()
			return
		}
	}
}

func (this *Connector) getDeviceId(device model.ZigbeeDeviceInfo) string {
	return this.config.DeviceIdPrefix + device.IeeeAddress
}

func (this *Connector) getDeviceIeeeAddress(deviceId string) (ieee string, prefixFound bool) {
	if strings.HasPrefix(deviceId, this.config.DeviceIdPrefix) {
		return strings.TrimPrefix(deviceId, this.config.DeviceIdPrefix), true
	}
	return "", false
}

func (this *Connector) getDeviceName(device model.ZigbeeDeviceInfo) string {
	return device.FriendlyName
}

func (this *Connector) getDeviceState(device model.ZigbeeDeviceInfo) mgw.State {
	if device.Disabled {
		return mgw.Offline
	} else {
		return mgw.Online
	}
}

func (this *Connector) getDeviceTypeId(device model.ZigbeeDeviceInfo) (string, error) {
	return this.devicerepo.FindDeviceTypeId(device)
}
