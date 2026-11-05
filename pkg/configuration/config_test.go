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

package configuration

import (
	"log/slog"
	"sync"
	"testing"
)

// the connector goroutines log their start through the same config at the same time; run with -race
func TestGetLoggerConcurrent(t *testing.T) {
	config := Config{Debug: true}
	loggers := make([]*slog.Logger, 10)
	wg := sync.WaitGroup{}
	for i := range loggers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loggers[i] = config.GetLogger()
		}()
	}
	wg.Wait()
	for i, logger := range loggers {
		if logger == nil || logger != loggers[0] {
			t.Errorf("logger %v differs: %p != %p", i, logger, loggers[0])
		}
	}
	if config.LogLevel != "" {
		t.Errorf("LogLevel changed to %q", config.LogLevel)
	}
}
