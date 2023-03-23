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
	"log"
	"sync"
)

func (this *Connector) startEventHandling(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("start event handling")
		for {
			select {
			case <-ctx.Done():
				log.Println("stop event handling")
				return
			case event := <-this.eventbuffer:
				this.handleEvent(event)
			}
		}
	}()
	return nil
}

func (this *Connector) handleEvent(event EventDesc) {
	if this.eventIsAllowed(event) {
		deviceid := this.getDeviceId(event.Device)
		err := this.mgw.SendEvent(deviceid, "get", event.Payload)
		if err != nil {
			log.Println("ERROR: unable to send event to mgw", err)
			this.mgw.SendDeviceError(deviceid, "unable to send event to mgw: "+err.Error())
		}
	}
}

func (this *Connector) eventIsAllowed(event EventDesc) bool {
	return !event.Device.Disabled && event.Device.Supported && event.Device.InterviewCompleted && this.deviceIsOnline(event.Device.IeeeAddress)
}
