package pkggomicro

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-micro/plugins/v4/client/grpc"
	"github.com/go-micro/plugins/v4/registry/consul"
	gmclient "go-micro.dev/v4/client"
	"go-micro.dev/v4/registry"

	defs "github.com/devpablocristo/customer-manager/pkg/microservices/go-micro/v4/grpc-client/defs"
)

var (
	instance   defs.Client
	once       sync.Once
	instanceMu sync.RWMutex
	initError  error
)

type client struct {
	c  gmclient.Client
	rs []*registry.Service
}

// newClient creates a gRPC client using the provided configuration and Consul as the service registry
func newClient(config defs.Config) (defs.Client, error) {
	once.Do(func() {
		clt, err := setupClient(config)
		if err != nil {
			initError = fmt.Errorf("error setting up gRPC client: %w", err)
			return
		}
		instance = &client{
			c: clt,
		}

		go attemptGettingRegisteredServices(config)
	})

	// NOTE: Este bucle de espera está diseñado para bloquear la ejecución de la función newClient hasta que la variable instance haya sido inicializada por la goroutine attemptConnection
	for {
		instanceMu.RLock()
		if instance != nil {
			instanceMu.RUnlock()
			break
		}
		instanceMu.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}

	return instance, nil
}

func attemptGettingRegisteredServices(config defs.Config) {
	for {
		fmt.Println("Getting gRPC server...")

		regSrv, err := getRegisteredServices(config)
		if err != nil {
			fmt.Printf("Error getting registered services from Consul: %v. Retrying in 5 seconds...\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		instanceMu.Lock()
		instance = &client{
			rs: regSrv,
		}
		instanceMu.Unlock()

		fmt.Println("Service found")
		break
	}
}

func getRegisteredServices(config defs.Config) ([]*registry.Service, error) {
	consulRegistry := consul.NewRegistry(registry.Addrs(config.GetConsulAddress()))

	services, err := consulRegistry.GetService(config.GetServerName())
	if err != nil {
		return nil, fmt.Errorf("error retrieving service %s from Consul: %w", config.GetServerName(), err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no instances found for service %s", config.GetServerName())
	}

	for _, service := range services {
		fmt.Printf("Service: %s\n", service.Name)
		// Iterate over service nodes
		for _, node := range service.Nodes {
			fmt.Printf("  - Instance ID: %s\n", node.Id)
			fmt.Printf("  - Address: %s\n", node.Address)

			// Try to access the port if available in metadata
			if port, ok := node.Metadata["port"]; ok {
				fmt.Printf("  - Port: %s\n", port)
			} else {
				fmt.Println("  - Port: Not available in metadata")
			}
		}
	}

	return services, nil
}

func setupClient(config defs.Config) (gmclient.Client, error) {
	consulRegistry := consul.NewRegistry(registry.Addrs(config.GetConsulAddress()))

	c := grpc.NewClient(
		gmclient.Registry(consulRegistry), // Use Consul for service discovery
	)

	return c, nil
}

// GetClient returns the configured gRPC client
func (c *client) GetClient() gmclient.Client {
	return c.c
}

func (c *client) GetServerName() string {
	return c.rs[0].Name
}