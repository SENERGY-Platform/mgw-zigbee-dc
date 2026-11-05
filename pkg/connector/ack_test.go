/*
 * Copyright (c) 2026 InfAI (CC SES)
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
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
)

func TestParseCommandAckTimeout(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"-", 0, false},
		{" - ", 0, false},
		{"0s", 0, false},
		{"30s", 30 * time.Second, false},
		{"1m", time.Minute, false},
		{"abc", 0, true},
		{"30", 0, true},
		{"-5s", 0, true},
	} {
		got, err := parseCommandAckTimeout(configuration.Config{CommandAckTimeout: tc.value})
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("%q: got %v, %v; want %v, error=%v", tc.value, got, err, tc.want, tc.wantErr)
		}
	}
}

var testExposes = []model.ZigbeeDeviceInfoFeature{
	{Type: "light", Features: []model.ZigbeeDeviceInfoFeature{
		{Type: "binary", Property: "state", Access: 7},
		{Type: "numeric", Property: "brightness", Access: 7},
		{Type: "composite", Property: "color", Access: 7, Features: []model.ZigbeeDeviceInfoFeature{
			{Type: "numeric", Property: "x", Access: 7},
		}},
	}},
	{Type: "enum", Property: "effect", Access: 2},
	{Type: "numeric", Property: "linkquality", Access: 1},
}

func TestAckExpectation(t *testing.T) {
	for name, tc := range map[string]struct {
		data    string
		exposes []model.ZigbeeDeviceInfoFeature
		want    map[string]interface{}
		wantOk  bool
	}{
		"plain":                          {`{"state":"ON","brightness":100}`, testExposes, map[string]interface{}{"state": "ON", "brightness": float64(100)}, true},
		"transition excluded":            {`{"state":"ON","transition":2}`, testExposes, map[string]interface{}{"state": "ON"}, true},
		"only transition":                {`{"transition":2}`, testExposes, nil, false},
		"unpublished excluded":           {`{"state":"ON","effect":"blink"}`, testExposes, map[string]interface{}{"state": "ON"}, true},
		"only unpublished":               {`{"effect":"blink"}`, testExposes, nil, false},
		"not exposed excluded":           {`{"state":"ON","brightness_percent":50}`, testExposes, map[string]interface{}{"state": "ON"}, true},
		"only not exposed":               {`{"brightness_percent":50}`, testExposes, nil, false},
		"nested property":                {`{"color":{"x":0.3}}`, testExposes, map[string]interface{}{"color": map[string]interface{}{"x": 0.3}}, true},
		"no exposes: all but transition": {`{"effect":"blink","foo":1,"transition":1}`, nil, map[string]interface{}{"effect": "blink", "foo": float64(1)}, true},
		"not an object":                  {`[1,2]`, testExposes, nil, false},
		"not json":                       {`on`, testExposes, nil, false},
		"empty":                          {``, testExposes, nil, false},
		"empty object":                   {`{}`, testExposes, nil, false},
		"json null":                      {`null`, testExposes, nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := ackExpectation(tc.data, tc.exposes)
			if ok != tc.wantOk {
				t.Fatalf("ok: got %v, want %v (%v)", ok, tc.wantOk, got)
			}
			if ok && !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestAckMatches(t *testing.T) {
	for name, tc := range map[string]struct {
		expected string
		state    string
		want     bool
	}{
		"equal string":             {`{"state":"ON"}`, `{"state":"ON","brightness":1}`, true},
		"string case insensitive":  {`{"state":"on"}`, `{"state":"ON"}`, true},
		"string differs":           {`{"state":"ON"}`, `{"state":"OFF"}`, false},
		"int vs float":             {`{"brightness":100}`, `{"brightness":100.0}`, true},
		"float vs int":             {`{"brightness":100.0}`, `{"brightness":100}`, true},
		"number differs":           {`{"brightness":100}`, `{"brightness":99}`, false},
		"number vs string":         {`{"brightness":100}`, `{"brightness":"100"}`, false},
		"bool equal":               {`{"x":true}`, `{"x":true}`, true},
		"bool differs":             {`{"x":true}`, `{"x":false}`, false},
		"toggle matches ON":        {`{"state":"TOGGLE"}`, `{"state":"ON"}`, true},
		"toggle matches OFF":       {`{"state":"toggle"}`, `{"state":"OFF"}`, true},
		"toggle needs the key":     {`{"state":"toggle"}`, `{"brightness":1}`, false},
		"missing key":              {`{"state":"ON","brightness":1}`, `{"state":"ON"}`, false},
		"all keys needed":          {`{"state":"ON","brightness":1}`, `{"state":"ON","brightness":2}`, false},
		"nested expected presence": {`{"color":{"x":0.3}}`, `{"color":{"x":0.9,"y":0.1}}`, true},
		"nested missing":           {`{"color":{"x":0.3}}`, `{"state":"ON"}`, false},
		"object published":         {`{"color":"red"}`, `{"color":{"x":1}}`, true},
		"array expected":           {`{"l":[1]}`, `{"l":[2]}`, true},
		"null published":           {`{"state":"ON"}`, `{"state":null}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			var exp, state map[string]interface{}
			if err := json.Unmarshal([]byte(tc.expected), &exp); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.state), &state); err != nil {
				t.Fatal(err)
			}
			if got := ackMatches(exp, state); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

type ackRecorder struct {
	mux       sync.Mutex
	responses []mgw.Command
	deviceIds []string
	errors    map[string]string
}

func newAckRecorder() *ackRecorder { return &ackRecorder{errors: map[string]string{}} }

func (this *ackRecorder) respond(deviceId string, serviceId string, command mgw.Command) error {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.responses = append(this.responses, command)
	this.deviceIds = append(this.deviceIds, deviceId+"/"+serviceId)
	return nil
}

func (this *ackRecorder) fail(commandId string, msg string) {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.errors[commandId] = msg
}

func (this *ackRecorder) snapshot() (responses []mgw.Command, errs map[string]string) {
	this.mux.Lock()
	defer this.mux.Unlock()
	errs = map[string]string{}
	for k, v := range this.errors {
		errs[k] = v
	}
	return append([]mgw.Command{}, this.responses...), errs
}

func newTestAckRegistry(t *testing.T, timeout time.Duration) (*ackRegistry, *ackRecorder, context.CancelFunc, *sync.WaitGroup) {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	rec := newAckRecorder()
	reg := newAckRegistry(ctx, wg, timeout, slog.Default(), rec.respond, rec.fail)
	t.Cleanup(func() { cancel(); wg.Wait() })
	return reg, rec, cancel, wg
}

func TestAckRegistryResponseOnMatch(t *testing.T) {
	reg, rec, _, _ := newTestAckRegistry(t, 500*time.Millisecond)
	reg.Register("ieee1", "zigbee:ieee1", "set", "c1", map[string]interface{}{"state": "ON"})
	reg.Register("ieee1", "zigbee:ieee1", "set", "c2", map[string]interface{}{"brightness": float64(5)})
	reg.Register("ieee2", "zigbee:ieee2", "set", "c3", map[string]interface{}{"state": "ON"})

	reg.Observe("ieee1", []byte(`{"state":"OFF"}`)) // matches nothing
	reg.Observe("ieee1", []byte(`not json`))
	reg.Observe("ieee1", []byte(`[1]`))
	time.Sleep(50 * time.Millisecond)
	if responses, _ := rec.snapshot(); len(responses) != 0 {
		t.Fatalf("unexpected responses %v", responses)
	}

	payload := `{"state":"on","brightness":5}` // satisfies c1 and c2, one message several entries
	reg.Observe("ieee1", []byte(payload))
	waitFor(t, func() bool { r, _ := rec.snapshot(); return len(r) == 2 })
	responses, errs := rec.snapshot()
	got := map[string]string{}
	for _, r := range responses {
		got[r.CommandId] = r.Data
	}
	if !reflect.DeepEqual(got, map[string]string{"c1": payload, "c2": payload}) {
		t.Errorf("responses: %v", got)
	}
	if len(errs) != 0 {
		t.Errorf("errors: %v", errs)
	}
	for _, id := range rec.deviceIds {
		if id != "zigbee:ieee1/set" {
			t.Errorf("response to %v", id)
		}
	}

	// matched entries are gone; the entry of another device times out and is the only one left
	reg.Observe("ieee1", []byte(payload))
	time.Sleep(50 * time.Millisecond)
	if responses, _ := rec.snapshot(); len(responses) != 2 {
		t.Errorf("entry answered twice: %v", responses)
	}
	waitFor(t, func() bool { _, e := rec.snapshot(); return e["c3"] != "" })
	responses, errs = rec.snapshot()
	if len(responses) != 2 || len(errs) != 1 {
		t.Errorf("responses %v errors %v", responses, errs)
	}
}

func TestAckRegistryTimeout(t *testing.T) {
	reg, rec, _, _ := newTestAckRegistry(t, 50*time.Millisecond)
	reg.Register("ieee1", "zigbee:ieee1", "set", "c1", map[string]interface{}{"state": "ON", "brightness": float64(5)})
	reg.Observe("ieee1", []byte(`{"state":"OFF"}`))
	waitFor(t, func() bool { _, e := rec.snapshot(); return len(e) == 1 })
	responses, errs := rec.snapshot()
	if len(responses) != 0 {
		t.Errorf("response on timeout: %v", responses)
	}
	msg := errs["c1"]
	for _, part := range []string{"zigbee:ieee1", "brightness, state", "50ms"} {
		if !strings.Contains(msg, part) {
			t.Errorf("error %q lacks %q", msg, part)
		}
	}
	// a state after the timeout answers nothing
	reg.Observe("ieee1", []byte(`{"state":"ON","brightness":5}`))
	time.Sleep(50 * time.Millisecond)
	if responses, _ := rec.snapshot(); len(responses) != 0 {
		t.Errorf("response after timeout: %v", responses)
	}
	if reg.hasPending("ieee1") {
		t.Error("entry not removed")
	}
}

func TestAckRegistryCancelAndDoesNotBlock(t *testing.T) {
	reg, rec, _, _ := newTestAckRegistry(t, 50*time.Millisecond)

	// cancel removes without any answer
	cancel := reg.Register("ieee1", "zigbee:ieee1", "set", "c1", map[string]interface{}{"state": "ON"})
	if !cancel() || cancel() {
		t.Error("cancel must report true once, then false")
	}
	time.Sleep(120 * time.Millisecond)
	responses, errs := rec.snapshot()
	if len(responses) != 0 || len(errs) != 0 {
		t.Errorf("answer after cancel: %v %v", responses, errs)
	}

	// Register and Observe return although respond blocks
	block := make(chan struct{})
	blocked := newAckRegistry(context.Background(), &sync.WaitGroup{}, time.Second, slog.Default(),
		func(string, string, mgw.Command) error { <-block; return nil },
		func(string, string) {})
	defer close(block)
	done := make(chan struct{})
	go func() {
		defer close(done)
		blocked.Register("ieee1", "d", "set", "c1", map[string]interface{}{"state": "ON"})
		blocked.Observe("ieee1", []byte(`{"state":"ON"}`))
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Register/Observe blocked on respond")
	}
}

func TestAckRegistryRespondError(t *testing.T) {
	rec := newAckRecorder()
	reg := newAckRegistry(context.Background(), &sync.WaitGroup{}, time.Second, slog.Default(),
		func(string, string, mgw.Command) error { return errors.New("offline") }, rec.fail)
	reg.Register("ieee1", "d", "set", "c1", map[string]interface{}{"state": "ON"})
	reg.Observe("ieee1", []byte(`{"state":"ON"}`))
	waitFor(t, func() bool { _, e := rec.snapshot(); return strings.Contains(e["c1"], "offline") })
}

func TestAckRegistryContextCancelStopsTimers(t *testing.T) {
	reg, rec, cancel, wg := newTestAckRegistry(t, 100*time.Millisecond)
	reg.Register("ieee1", "d", "set", "c1", map[string]interface{}{"state": "ON"})
	cancel()
	wg.Wait() // the watcher goroutine ends
	time.Sleep(250 * time.Millisecond)
	if responses, errs := rec.snapshot(); len(responses) != 0 || len(errs) != 0 {
		t.Errorf("answer after cancel: %v %v", responses, errs)
	}
	if reg.hasPending("ieee1") {
		t.Error("entries kept after cancel")
	}
	// registering on a closed registry neither panics nor answers
	reg.Register("ieee1", "d", "set", "c2", map[string]interface{}{"state": "ON"})
	time.Sleep(150 * time.Millisecond)
	if _, errs := rec.snapshot(); len(errs) != 0 {
		t.Errorf("answer from closed registry: %v", errs)
	}
}

func TestAckRegistryConcurrent(t *testing.T) {
	reg, rec, _, _ := newTestAckRegistry(t, time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			reg.Register("ieee1", "d", "set", "c", map[string]interface{}{"state": "ON"})
		}()
		go func() {
			defer wg.Done()
			reg.Observe("ieee1", []byte(`{"state":"ON"}`))
		}()
	}
	wg.Wait()
	reg.Observe("ieee1", []byte(`{"state":"ON"}`))
	waitFor(t, func() bool { r, _ := rec.snapshot(); return len(r) == 50 })
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

type fakeSetZigbee struct {
	ZigbeeClient
	devices []model.ZigbeeDeviceInfo
	setErr  error
	onSet   func()
}

func (this *fakeSetZigbee) SetByIeee(string, []byte) error {
	if this.onSet != nil {
		this.onSet()
	}
	return this.setErr
}

func (this *fakeSetZigbee) GetDeviceList(context.Context) ([]model.ZigbeeDeviceInfo, error) {
	return this.devices, nil
}

type fakeSetMgw struct {
	MgwClient
	rec *ackRecorder
}

func (this *fakeSetMgw) Respond(deviceId string, serviceId string, command mgw.Command) error {
	return this.rec.respond(deviceId, serviceId, command)
}

func (this *fakeSetMgw) SendCommandError(commandId string, msg string) { this.rec.fail(commandId, msg) }

func TestHandleSet(t *testing.T) {
	newConnector := func(t *testing.T, timeout string, zb *fakeSetZigbee) (*Connector, *ackRecorder) {
		rec := newAckRecorder()
		c := &Connector{config: configuration.Config{DeviceIdPrefix: "zigbee:"}, zigbee: zb, mgw: &fakeSetMgw{rec: rec}}
		d, err := parseCommandAckTimeout(configuration.Config{CommandAckTimeout: timeout})
		if err != nil {
			t.Fatal(err)
		}
		if d > 0 {
			ctx, cancel := context.WithCancel(context.Background())
			wg := &sync.WaitGroup{}
			c.acks = newAckRegistry(ctx, wg, d, slog.Default(), c.mgw.Respond, c.mgw.SendCommandError)
			t.Cleanup(func() { cancel(); wg.Wait() })
		}
		return c, rec
	}
	device := model.ZigbeeDeviceInfo{IeeeAddress: "ieee1", Definition: model.ZigbeeDeviceInfoDefinition{Exposes: testExposes}}
	cmd := func(data string) CommandDesc {
		return CommandDesc{DeviceId: "zigbee:ieee1", ServiceId: "set", Command: mgw.Command{CommandId: "c1", Data: data}}
	}

	t.Run("disabled responds immediately and empty", func(t *testing.T) {
		c, rec := newConnector(t, "", &fakeSetZigbee{devices: []model.ZigbeeDeviceInfo{device}})
		c.handleCommand(cmd(`{"state":"ON"}`))
		responses, errs := rec.snapshot()
		if len(responses) != 1 || responses[0].Data != "" || len(errs) != 0 {
			t.Errorf("%v %v", responses, errs)
		}
	})

	t.Run("no comparable key responds immediately and empty", func(t *testing.T) {
		for _, data := range []string{`{"transition":1}`, `{"effect":"blink"}`, `[1]`, `on`} {
			c, rec := newConnector(t, "1s", &fakeSetZigbee{devices: []model.ZigbeeDeviceInfo{device}})
			c.handleCommand(cmd(data))
			responses, errs := rec.snapshot()
			if len(responses) != 1 || responses[0].Data != "" || len(errs) != 0 {
				t.Errorf("%v: %v %v", data, responses, errs)
			}
		}
	})

	t.Run("state before publish returns is not missed, response carries the state", func(t *testing.T) {
		zb := &fakeSetZigbee{devices: []model.ZigbeeDeviceInfo{device}}
		c, rec := newConnector(t, "1s", zb)
		zb.onSet = func() { c.StateObserved("ieee1", []byte(`{"state":"ON"}`)) } // fast state while publishing
		c.handleCommand(cmd(`{"state":"ON","transition":1}`))
		waitFor(t, func() bool { r, _ := rec.snapshot(); return len(r) == 1 })
		responses, errs := rec.snapshot()
		if responses[0].Data != `{"state":"ON"}` || responses[0].CommandId != "c1" || len(errs) != 0 {
			t.Errorf("%v %v", responses, errs)
		}
	})

	t.Run("waits for the state without blocking", func(t *testing.T) {
		c, rec := newConnector(t, "1s", &fakeSetZigbee{devices: []model.ZigbeeDeviceInfo{device}})
		c.handleCommand(cmd(`{"state":"ON"}`))
		if responses, errs := rec.snapshot(); len(responses) != 0 || len(errs) != 0 {
			t.Fatalf("answered before the state: %v %v", responses, errs)
		}
		c.StateObserved("ieee1", []byte(`{"state":"OFF"}`))
		time.Sleep(30 * time.Millisecond)
		if responses, _ := rec.snapshot(); len(responses) != 0 {
			t.Fatalf("answered by a non matching state: %v", responses)
		}
		c.StateObserved("ieee1", []byte(`{"state":"ON","linkquality":1}`))
		waitFor(t, func() bool { r, _ := rec.snapshot(); return len(r) == 1 })
	})

	t.Run("publish error removes the entry", func(t *testing.T) {
		c, rec := newConnector(t, "50ms", &fakeSetZigbee{devices: []model.ZigbeeDeviceInfo{device}, setErr: errors.New("broker down")})
		c.handleCommand(cmd(`{"state":"ON"}`))
		time.Sleep(150 * time.Millisecond)
		responses, errs := rec.snapshot()
		if len(responses) != 0 || len(errs) != 1 || !strings.Contains(errs["c1"], "broker down") {
			t.Errorf("%v %v", responses, errs)
		}
		if c.acks.hasPending("ieee1") {
			t.Error("entry kept")
		}
	})

	t.Run("publish error after a matching state answers only once", func(t *testing.T) {
		zb := &fakeSetZigbee{devices: []model.ZigbeeDeviceInfo{device}, setErr: errors.New("broker down")}
		c, rec := newConnector(t, "1s", zb)
		zb.onSet = func() {
			c.StateObserved("ieee1", []byte(`{"state":"ON"}`))
			time.Sleep(50 * time.Millisecond) // let the response go out
		}
		c.handleCommand(cmd(`{"state":"ON"}`))
		waitFor(t, func() bool { r, _ := rec.snapshot(); return len(r) == 1 })
		time.Sleep(50 * time.Millisecond)
		responses, errs := rec.snapshot()
		if len(responses) != 1 || len(errs) != 0 {
			t.Errorf("%v %v", responses, errs)
		}
	})

	t.Run("timeout is a command error", func(t *testing.T) {
		c, rec := newConnector(t, "50ms", &fakeSetZigbee{devices: []model.ZigbeeDeviceInfo{device}})
		c.handleCommand(cmd(`{"state":"ON"}`))
		waitFor(t, func() bool { _, e := rec.snapshot(); return len(e) == 1 })
		if responses, _ := rec.snapshot(); len(responses) != 0 {
			t.Errorf("%v", responses)
		}
	})

	t.Run("unknown device definition compares every key", func(t *testing.T) {
		c, rec := newConnector(t, "1s", &fakeSetZigbee{})
		c.handleCommand(cmd(`{"effect":"blink"}`))
		if responses, _ := rec.snapshot(); len(responses) != 0 {
			t.Fatalf("answered immediately: %v", responses)
		}
		c.StateObserved("ieee1", []byte(`{"effect":"blink"}`))
		waitFor(t, func() bool { r, _ := rec.snapshot(); return len(r) == 1 })
	})
}
