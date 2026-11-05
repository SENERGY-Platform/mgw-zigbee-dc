/*
 * Copyright 2026 InfAI (CC SES)
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
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
)

// retryRepo fails every read while down, as the device repository mirror does while it restarts
type retryRepo struct {
	DeviceRepo
	down  atomic.Bool
	reads atomic.Int32
}

// a test that waits for reads sees the outcome of each counted read decided
func (this *retryRepo) GetKnownDeviceDeviceTypeId(string) (string, bool, error) {
	down := this.down.Load()
	this.reads.Add(1)
	if down {
		return "", false, errors.New("connect: connection refused")
	}
	return "", false, nil
}

func (this *retryRepo) FindDeviceTypeId(device model.ZigbeeDeviceInfo) (string, error) {
	if device.Definition.Model == "unknown" {
		return "", fmt.Errorf("%w: model=unknown", model.NoMatchingDeviceTypeFound)
	}
	return "dt-" + device.Definition.Model, nil
}

type retryMgw struct {
	MgwClient
	mux          sync.Mutex
	devices      map[string]mgw.DeviceInfo
	clientErrors int
}

func (this *retryMgw) SetDevice(deviceId string, info mgw.DeviceInfo) error {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.devices[deviceId] = info
	return nil
}

func (this *retryMgw) SendClientError(string) {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.clientErrors++
}

func (this *retryMgw) snapshot() (devices map[string]mgw.DeviceInfo, clientErrors int) {
	this.mux.Lock()
	defer this.mux.Unlock()
	devices = map[string]mgw.DeviceInfo{}
	for k, v := range this.devices {
		devices[k] = v
	}
	return devices, this.clientErrors
}

type retryZigbee struct {
	ZigbeeClient
	mux       sync.Mutex
	refreshed []string
}

func (this *retryZigbee) RefreshDeviceEventValues(device model.ZigbeeDeviceInfo) error {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.refreshed = append(this.refreshed, device.IeeeAddress)
	return nil
}

func (this *retryZigbee) GetZigbeeMessageOutputStruct(model.ZigbeeDeviceInfo) map[string]interface{} {
	return nil
}

func (this *retryZigbee) GetZigbeeMessageInputStruct(model.ZigbeeDeviceInfo) map[string]interface{} {
	return nil
}

func (this *retryZigbee) snapshot() []string {
	this.mux.Lock()
	defer this.mux.Unlock()
	return slices.Clone(this.refreshed)
}

func TestDeviceRetryWhileRepositoryIsDown(t *testing.T) {
	newDevice := func(ieee string, m string) model.ZigbeeDeviceInfo {
		return model.ZigbeeDeviceInfo{IeeeAddress: ieee, FriendlyName: ieee, Supported: true, InterviewCompleted: true,
			Definition: model.ZigbeeDeviceInfoDefinition{Vendor: "vendor", Model: m, Exposes: testExposes}}
	}
	start := func(t *testing.T, repo *retryRepo) (*Connector, *retryMgw, *retryZigbee) {
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		t.Cleanup(func() { cancel(); wg.Wait() })
		mgwClient := &retryMgw{devices: map[string]mgw.DeviceInfo{}}
		zb := &retryZigbee{}
		c := &Connector{
			config:             configuration.Config{DeviceIdPrefix: "zigbee:"},
			deviceupdatebuffer: make(chan []model.ZigbeeDeviceInfo, 10),
			devicerepo:         repo,
			devicestate:        map[string]mgw.State{},
			mgw:                mgwClient,
			zigbee:             zb,
		}
		err := c.startDeviceHandling(ctx, wg)
		if err != nil {
			t.Fatal(err)
		}
		return c, mgwClient, zb
	}

	t.Run("devices are registered once the repository is readable", func(t *testing.T) {
		repo := &retryRepo{}
		repo.down.Store(true)
		c, mgwClient, zb := start(t, repo)
		d1, d2, disabled := newDevice("0x01", "m1"), newDevice("0x02", "m2"), newDevice("0x03", "m3")
		disabled.Disabled = true
		c.DeviceInfoUpdate([]model.ZigbeeDeviceInfo{d1, d2, disabled})
		waitFor(t, func() bool { return repo.reads.Load() >= 3 })
		if devices, _ := mgwClient.snapshot(); len(devices) != 0 || c.eventIsAllowed(EventDesc{Device: d1}) {
			t.Fatalf("registered while the repository is down: %v", devices)
		}

		repo.down.Store(false)
		waitFor(t, func() bool { devices, _ := mgwClient.snapshot(); return len(devices) == 3 })
		devices, _ := mgwClient.snapshot()
		if devices["zigbee:0x01"].DeviceType != "dt-m1" || devices["zigbee:0x02"].DeviceType != "dt-m2" || devices["zigbee:0x03"].State != mgw.Offline {
			t.Errorf("%v", devices)
		}
		for _, d := range []model.ZigbeeDeviceInfo{d1, d2} {
			if !c.eventIsAllowed(EventDesc{Device: d}) {
				t.Errorf("events of %v are not allowed", d.IeeeAddress)
			}
		}
		// the events of the initial refresh were dropped, so the values of the enabled devices are requested again
		waitFor(t, func() bool { return len(zb.snapshot()) >= 2 })
		if refreshed := zb.snapshot(); !slices.Equal(refreshed, []string{"0x01", "0x02"}) {
			t.Errorf("%v", refreshed)
		}
	})

	t.Run("a new device list replaces the devices to retry", func(t *testing.T) {
		repo := &retryRepo{}
		repo.down.Store(true)
		c, mgwClient, zb := start(t, repo)
		c.DeviceInfoUpdate([]model.ZigbeeDeviceInfo{newDevice("0x01", "m1")})
		c.DeviceInfoUpdate([]model.ZigbeeDeviceInfo{newDevice("0x02", "m2")})
		waitFor(t, func() bool { return repo.reads.Load() >= 2 })

		repo.down.Store(false)
		waitFor(t, func() bool { devices, _ := mgwClient.snapshot(); return len(devices) == 1 })
		devices, _ := mgwClient.snapshot()
		if _, ok := devices["zigbee:0x02"]; !ok {
			t.Errorf("%v", devices)
		}
		waitFor(t, func() bool { return len(zb.snapshot()) >= 1 })
		if refreshed := zb.snapshot(); !slices.Equal(refreshed, []string{"0x02"}) {
			t.Errorf("%v", refreshed)
		}
	})

	t.Run("a missing device type is not retried", func(t *testing.T) {
		repo := &retryRepo{}
		c, mgwClient, zb := start(t, repo)
		c.DeviceInfoUpdate([]model.ZigbeeDeviceInfo{newDevice("0x03", "unknown")})
		waitFor(t, func() bool { _, clientErrors := mgwClient.snapshot(); return clientErrors == 1 })
		time.Sleep(deviceRetryMinDelay + 500*time.Millisecond)
		if _, clientErrors := mgwClient.snapshot(); clientErrors != 1 || repo.reads.Load() != 1 || len(zb.snapshot()) != 0 {
			t.Errorf("client errors %v, reads %v, refreshed %v", clientErrors, repo.reads.Load(), zb.snapshot())
		}
	})
}
