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
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
)

// zigbeePublishedAccess is the bit of the zigbee2mqtt access field that marks a property as part of the published state
const zigbeePublishedAccess = 0b001

// zigbeeSetTransitionKey is a set option that never appears in the published state
const zigbeeSetTransitionKey = "transition"

// parseCommandAckTimeout returns the configured acknowledgement timeout; 0 means the acknowledgement is disabled
// ("", "-" and a zero duration disable it, an unparsable or negative value is an error)
func parseCommandAckTimeout(config configuration.Config) (time.Duration, error) {
	value := strings.TrimSpace(config.CommandAckTimeout)
	if value == "" || value == "-" {
		return 0, nil
	}
	dur, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for CommandAckTimeout: %w", err)
	}
	if dur < 0 {
		return 0, fmt.Errorf("invalid duration for CommandAckTimeout: must not be negative, got %v", value)
	}
	return dur, nil
}

// ackExpectation returns the key/value pairs of a set payload that the published device state must show
// before the command counts as acknowledged. ok is false if the command can not be acknowledged by a state,
// the caller then answers immediately.
//
// exposes == nil means that the device definition is not available: every key except transition is comparable.
// Otherwise a key is comparable only if the definition exposes it, searched through nested features, with the
// published access bit; a key that is not exposed never appears in the state and would end in a timeout.
// A property without access information (access 0) is treated as published.
//
// lastState is the last state zigbee2mqtt published for the device, nil or empty if none is known. If it is known, a
// key is comparable only if it occurs in it: some definitions expose a property as published that the device never
// reports (Aqara SRTS-A01 external_temperature_input), and zigbee2mqtt's state carries every property it has cached
// (advanced.cache_state, the default). A property the device has not reported yet is therefore not confirmed.
func ackExpectation(data string, exposes []model.ZigbeeDeviceInfoFeature, lastState map[string]interface{}) (expected map[string]interface{}, ok bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, false
	}
	var published map[string]bool
	if len(exposes) > 0 {
		published = map[string]bool{}
		collectPublishedProperties(exposes, published)
	}
	expected = map[string]interface{}{}
	for key, value := range payload {
		if key == zigbeeSetTransitionKey {
			continue
		}
		if published != nil && !published[key] {
			continue
		}
		if len(lastState) > 0 {
			if _, reported := lastState[key]; !reported {
				continue
			}
		}
		expected[key] = value
	}
	return expected, len(expected) > 0
}

// collectPublishedProperties adds every property of the features, including nested ones, to result;
// a property that occurs several times is published if any occurrence is
func collectPublishedProperties(features []model.ZigbeeDeviceInfoFeature, result map[string]bool) {
	for _, feature := range features {
		if feature.Property != "" {
			if feature.Access == 0 || feature.Access&zigbeePublishedAccess != 0 {
				result[feature.Property] = true
			} else if _, seen := result[feature.Property]; !seen {
				result[feature.Property] = false
			}
		}
		collectPublishedProperties(feature.Features, result)
	}
}

// ackMatches checks if a published state contains every expected key with a matching value
func ackMatches(expected map[string]interface{}, state map[string]interface{}) bool {
	for key, exp := range expected {
		got, ok := state[key]
		if !ok {
			return false
		}
		if !ackValueMatches(exp, got) {
			return false
		}
	}
	return true
}

func ackValueMatches(expected interface{}, got interface{}) bool {
	switch exp := expected.(type) {
	case map[string]interface{}, []interface{}:
		return true
	case string:
		if strings.EqualFold(exp, "toggle") {
			return true
		}
		switch g := got.(type) {
		case string:
			return strings.EqualFold(exp, g)
		case map[string]interface{}, []interface{}:
			return true
		}
		return false
	case float64:
		switch g := got.(type) {
		case float64:
			return exp == g
		case map[string]interface{}, []interface{}:
			return true
		}
		return false
	case bool:
		switch g := got.(type) {
		case bool:
			return exp == g
		case map[string]interface{}, []interface{}:
			return true
		}
		return false
	case nil:
		switch got.(type) {
		case nil, map[string]interface{}, []interface{}:
			return true
		}
		return false
	}
	return false
}

type pendingAck struct {
	ieee      string
	deviceId  string
	serviceId string
	commandId string
	expected  map[string]interface{}
	deadline  time.Time
	timer     *time.Timer
}

// ackRegistry holds the set commands that wait for a matching device state.
// It never blocks the caller of Register or Observe: matches and timeouts are answered in their own goroutines.
type ackRegistry struct {
	timeout time.Duration
	respond func(deviceId string, serviceId string, command mgw.Command) error
	fail    func(commandId string, msg string)
	logger  *slog.Logger
	wg      *sync.WaitGroup

	mux     sync.Mutex
	pending map[string][]*pendingAck // by ieee address
	closed  bool
}

func newAckRegistry(ctx context.Context, wg *sync.WaitGroup, timeout time.Duration, logger *slog.Logger, respond func(deviceId string, serviceId string, command mgw.Command) error, fail func(commandId string, msg string)) *ackRegistry {
	result := &ackRegistry{
		timeout: timeout,
		respond: respond,
		fail:    fail,
		logger:  logger,
		wg:      wg,
		pending: map[string][]*pendingAck{},
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		result.close()
	}()
	return result
}

func (this *ackRegistry) close() {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.closed = true
	for _, list := range this.pending {
		for _, entry := range list {
			entry.timer.Stop()
		}
	}
	this.pending = map[string][]*pendingAck{}
}

// Register adds a pending acknowledgement and starts its timeout; call it before the command is published.
// The returned function removes the entry again (e.g. if publishing failed) without any answer.
// It reports whether the entry was still registered; false means it was already answered (match or timeout).
func (this *ackRegistry) Register(ieee string, deviceId string, serviceId string, commandId string, expected map[string]interface{}) (cancel func() bool) {
	entry := &pendingAck{
		ieee:      ieee,
		deviceId:  deviceId,
		serviceId: serviceId,
		commandId: commandId,
		expected:  expected,
		deadline:  time.Now().Add(this.timeout),
	}
	this.mux.Lock()
	defer this.mux.Unlock()
	if this.closed {
		return func() bool { return true }
	}
	entry.timer = time.AfterFunc(this.timeout, func() { this.expire(entry) })
	this.pending[ieee] = append(this.pending[ieee], entry)
	return func() bool {
		this.mux.Lock()
		defer this.mux.Unlock()
		if this.removeLocked(entry) {
			entry.timer.Stop()
			return true
		}
		return false
	}
}

// removeLocked returns false if the entry was already removed
func (this *ackRegistry) removeLocked(entry *pendingAck) bool {
	list := this.pending[entry.ieee]
	for i, e := range list {
		if e == entry {
			rest := append(append([]*pendingAck{}, list[:i]...), list[i+1:]...)
			if len(rest) == 0 {
				delete(this.pending, entry.ieee)
			} else {
				this.pending[entry.ieee] = rest
			}
			return true
		}
	}
	return false
}

func (this *ackRegistry) hasPending(ieee string) bool {
	this.mux.Lock()
	defer this.mux.Unlock()
	return len(this.pending[ieee]) > 0
}

// Observe hands a published (non-retained) device state to the registry; it answers every pending command it satisfies
func (this *ackRegistry) Observe(ieee string, payload []byte) {
	if !this.hasPending(ieee) {
		return
	}
	var state map[string]interface{}
	if err := json.Unmarshal(payload, &state); err != nil {
		return
	}
	var matched []*pendingAck
	this.mux.Lock()
	if this.closed {
		this.mux.Unlock()
		return
	}
	for _, entry := range append([]*pendingAck{}, this.pending[ieee]...) {
		if ackMatches(entry.expected, state) && this.removeLocked(entry) {
			entry.timer.Stop()
			matched = append(matched, entry)
		}
	}
	// Add under the lock: close() sets closed under it and the watcher goroutine still holds its count then
	this.wg.Add(len(matched))
	this.mux.Unlock()
	data := string(payload)
	for _, entry := range matched {
		go func(entry *pendingAck) {
			defer this.wg.Done()
			this.deliver(entry, data)
		}(entry)
	}
}

func (this *ackRegistry) deliver(entry *pendingAck, data string) {
	err := this.respond(entry.deviceId, entry.serviceId, mgw.Command{CommandId: entry.commandId, Data: data})
	if err != nil {
		this.logger.Error("unable to send set response", "error", err)
		this.fail(entry.commandId, "unable to send set response: "+err.Error())
	}
}

func (this *ackRegistry) expire(entry *pendingAck) {
	this.mux.Lock()
	removed := this.removeLocked(entry)
	this.mux.Unlock()
	if !removed {
		return
	}
	keys := make([]string, 0, len(entry.expected))
	for key := range entry.expected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	this.fail(entry.commandId, fmt.Sprintf("no matching state of device %v within %v after set command, expected keys: %v", entry.deviceId, this.timeout, strings.Join(keys, ", ")))
}
