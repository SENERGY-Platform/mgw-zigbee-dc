package docker

import (
	"context"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/ory/dockertest/v3"
	"log"
	"math/rand"
	"strconv"
	"sync"
)

func Mqtt(ctx context.Context, wg *sync.WaitGroup) (hostPort string, ipAddress string, err error) {
	log.Println("start mqtt broker")
	pool, err := dockertest.NewPool("")
	if err != nil {
		return "", "", err
	}
	container, err := pool.Run("eclipse-mosquitto", "1.6.12", []string{})
	if err != nil {
		return "", "", err
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		log.Println("DEBUG: remove container " + container.Container.Name)
		log.Println(container.Close())
	}()
	go Dockerlog(pool, ctx, container, "MQTT-BROKER")
	hostPort = container.GetPort("1883/tcp")
	err = pool.Retry(func() error {
		log.Println("try to connection to broker...")
		options := paho.NewClientOptions().
			SetAutoReconnect(true).
			SetCleanSession(false).
			SetClientID("try-test-connection-" + strconv.Itoa(rand.Int())).
			AddBroker("tcp://localhost:" + hostPort)

		client := paho.NewClient(options)
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			log.Println("Error on Mqtt.Connect(): ", token.Error())
			return token.Error()
		}
		defer client.Disconnect(0)
		return nil
	})
	return hostPort, container.Container.NetworkSettings.IPAddress, err
}
