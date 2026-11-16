// Package registry resolves a worker type to its driver.
package registry

import (
	"fmt"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers/adapters"
)

var containerDrivers = map[config.WorkerType]workers.Driver{
	config.WorkerClaudeCode: adapters.ClaudeCode{},
	config.WorkerCodex:      adapters.NewCodex(false),
	config.WorkerPi:         adapters.NewPi(false),
	config.WorkerMock:       adapters.Mock{},
}

// Local drivers invoke the host CLIs in their native configuration. claudecode and
// mock build identical commands in both modes, so they are shared.
var localDrivers = map[config.WorkerType]workers.Driver{
	config.WorkerClaudeCode: adapters.ClaudeCode{},
	config.WorkerCodex:      adapters.NewCodex(true),
	config.WorkerPi:         adapters.NewPi(true),
	config.WorkerMock:       adapters.Mock{},
}

// Get returns the driver for a worker type and execution mode.
func Get(workerType config.WorkerType, execution config.ExecutionMode) (workers.Driver, error) {
	drivers := containerDrivers
	if execution == config.ExecutionLocal {
		drivers = localDrivers
	}
	driver, ok := drivers[workerType]
	if !ok {
		return nil, fmt.Errorf("unknown worker type: %s", workerType)
	}
	return driver, nil
}
