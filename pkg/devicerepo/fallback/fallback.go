/*
 * Copyright 2021 InfAI (CC SES)
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

package fallback

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
)

type Fallback interface {
	Get(key string) (value interface{}, err error)
	Set(key string, value interface{}) (err error)
}

func NewFallback(file string) (result *FallbackImpl, err error) {
	result = &FallbackImpl{
		file: file,
	}
	err = result.loadState()
	return
}

type FallbackImpl struct {
	file   string
	memory map[string]interface{}
	mux    sync.Mutex
}

func (this *FallbackImpl) Get(key string) (value interface{}, err error) {
	this.mux.Lock()
	defer this.mux.Unlock()
	if this.memory == nil {
		this.memory = map[string]interface{}{}
	}
	value, exists := this.memory[key]
	if !exists {
		err = errors.New("value not found in fallback: " + key)
	}
	return value, err
}

func (this *FallbackImpl) Set(key string, value interface{}) (err error) {
	this.mux.Lock()
	defer this.mux.Unlock()
	if this.memory == nil {
		this.memory = map[string]interface{}{}
	}
	this.memory[key] = value
	file, err := json.MarshalIndent(this.memory, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(this.file, file, 0644)
}

func (this *FallbackImpl) loadState() (err error) {
	this.mux.Lock()
	defer this.mux.Unlock()
	log.Println("load fallback file", this.file)
	exists, err := fileExists(this.file)
	if err != nil {
		return err
	}
	if !exists {
		log.Println("fall back file does not exist --> a new one will be created at", this.file)
		this.memory = map[string]interface{}{}
		return
	}
	temp, err := os.ReadFile(this.file)
	if err != nil {
		return err
	}
	state := map[string]interface{}{}
	err = json.Unmarshal(temp, &state)
	if err != nil {
		return err
	}
	this.memory = state
	return nil
}

func fileExists(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return false, errors.New("fallback file is dir")
	}
	return true, nil
}
