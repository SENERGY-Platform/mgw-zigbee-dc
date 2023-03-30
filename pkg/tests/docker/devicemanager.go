/*
 * Copyright 2020 InfAI (CC SES)
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

package docker

import (
	"context"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"log"
	"net/http"
	"sync"
)

func DeviceManager(ctx context.Context, wg *sync.WaitGroup, kafkaUrl string, devicerepo string, permsearch string) (hostPort string, ipAddress string, err error) {
	log.Println("start device-manager")
	pool, err := dockertest.NewPool("")
	if err != nil {
		return "", "", err
	}
	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "ghcr.io/senergy-platform/device-manager",
		Tag:        "dev",
		Env: []string{
			"KAFKA_URL=" + kafkaUrl,
			"DEVICE_REPO_URL=" + devicerepo,
			"PERMISSIONS_URL=" + permsearch,
			"DISABLE_VALIDATION=true",
		},
	}, func(config *docker.HostConfig) {
		config.RestartPolicy = docker.RestartPolicy{Name: "unless-stopped"}
	})
	if err != nil {
		return "", "", err
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		log.Println("DEBUG: remove container " + container.Container.Name)
		container.Close()
	}()
	go Dockerlog(pool, ctx, container, "DEVICE-MANAGER")
	hostPort = container.GetPort("8080/tcp")
	err = pool.Retry(func() error {
		log.Println("try device-manager connection...")
		_, err := http.Get("http://localhost:" + hostPort)
		if err != nil {
			log.Println(err)
		}
		return err
	})
	return hostPort, container.Container.NetworkSettings.IPAddress, err
}

func DeviceManagerWithDependencies(basectx context.Context, wg *sync.WaitGroup) (managerUrl string, repoUrl string, searchUrl string, err error) {
	_, managerUrl, repoUrl, searchUrl, err = DeviceManagerWithDependenciesAndKafka(basectx, wg)
	return
}

func DeviceManagerWithDependenciesAndKafka(basectx context.Context, wg *sync.WaitGroup) (kafkaUrl string, managerUrl string, repoUrl string, searchUrl string, err error) {
	ctx, cancel := context.WithCancel(basectx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	_, zkIp, err := Zookeeper(ctx, wg)
	if err != nil {
		return kafkaUrl, managerUrl, repoUrl, searchUrl, err
	}
	zookeeperUrl := zkIp + ":2181"

	kafkaUrl, err = Kafka(ctx, wg, zookeeperUrl)
	if err != nil {
		return kafkaUrl, managerUrl, repoUrl, searchUrl, err
	}

	_, elasticIp, err := Elasticsearch(ctx, wg)
	if err != nil {
		return kafkaUrl, managerUrl, repoUrl, searchUrl, err
	}

	_, permIp, err := PermSearch(ctx, wg, kafkaUrl, elasticIp)
	if err != nil {
		return kafkaUrl, managerUrl, repoUrl, searchUrl, err
	}
	searchUrl = "http://" + permIp + ":8080"

	_, mongoIp, err := MongoDB(ctx, wg)
	if err != nil {
		return kafkaUrl, managerUrl, repoUrl, searchUrl, err
	}

	_, repoIp, err := DeviceRepo(ctx, wg, kafkaUrl, "mongodb://"+mongoIp+":27017", searchUrl)
	if err != nil {
		return kafkaUrl, managerUrl, repoUrl, searchUrl, err
	}
	repoUrl = "http://" + repoIp + ":8080"

	_, managerIp, err := DeviceManager(ctx, wg, kafkaUrl, repoUrl, searchUrl)
	if err != nil {
		return kafkaUrl, managerUrl, repoUrl, searchUrl, err
	}
	managerUrl = "http://" + managerIp + ":8080"

	return kafkaUrl, managerUrl, repoUrl, searchUrl, err
}
