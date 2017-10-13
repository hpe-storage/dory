/*
(c) Copyright 2017 Hewlett Packard Enterprise Development LP

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chain

import (
	"fmt"
	"sync"
	"time"
)

// Chain is a set of Runners that will be executed sequentially
type Chain struct {
	maxRetryOnError  int
	sleepBeforeRetry time.Duration
	commands         []Runner
	output           map[string]interface{}
	outputLock       *sync.RWMutex
	step             int
	err              error
	rollbackErr      error
	runLock          *sync.Mutex
	done             bool
}

// Runner describes a struct that can be run and rolled back
type Runner interface {
	// Name is used for logging and locating the output of a previously run Runner
	Name() string
	// Run does the work and returns a value.  If error is returned the chain fails (after retries)
	Run() (interface{}, error)
	// Rollback is used to undo whatever Run() did
	Rollback() error
}

// NewChain creates a new chain.
// retries dictates how many times a Runner should be retried on error.
// retrySleep is how long to sleep before retrying a failed Runner
func NewChain(retries int, retrySleep time.Duration) *Chain {
	return &Chain{
		commands:         make([]Runner, 0),
		maxRetryOnError:  retries,
		sleepBeforeRetry: retrySleep,
		output:           make(map[string]interface{}),
		outputLock:       &sync.RWMutex{},
		runLock:          &sync.Mutex{},
	}
}

// AppendRunner appends a Runner to the Chain
func (c *Chain) AppendRunner(cmd Runner) error {
	c.runLock.Lock()
	defer c.runLock.Unlock()

	if c.done {
		return fmt.Errorf("this chain has already executed")
	}

	c.commands = append(c.commands, cmd)
	return nil
}

// Execute runs the chain exactly once
func (c *Chain) Execute() error {
	c.runLock.Lock()
	defer c.runLock.Unlock()

	err := c.setup()
	if err != nil {
		return err
	}

	c.done = true
	c.step = 0
	for i, command := range c.commands {
		if command == nil {
			continue
		}
		var out interface{}
		out, err = c.runWithRetry(i, command)
		if err != nil {
			c.err = err
			break
		}
		c.outputLock.Lock()
		c.output[command.Name()] = out
		c.outputLock.Unlock()
	}
	if c.err != nil {
		completed := c.commands[:c.step+1]
		for i := len(completed) - 1; i >= 0; i-- {
			if c.commands[i] == nil {
				continue
			}
			err := c.rollbackWithRetry(c.commands[i])
			if err != nil {
				c.rollbackErr = err
			}
		}
	}
	return c.err
}

// Error returns the last error returned by a Runner
func (c *Chain) Error() error {
	return c.err
}

//ErrorRollback returns the last error returned by a Runner
func (c *Chain) ErrorRollback() error {
	return c.rollbackErr
}

// GetRunnerOutput returns the output from a Runner.
// It is valid to pass *Chain to a Runner.  The
// Runner can then use *Chain.GetRunnerOutput(...) to reference
// the output of Runners that executed before them.
func (c *Chain) GetRunnerOutput(name string) interface{} {
	c.outputLock.RLock()
	defer c.outputLock.RUnlock()
	return c.output[name]
}

func (c *Chain) setup() error {
	if c.done {
		return fmt.Errorf("this chain has already executed")
	}

	for _, command := range c.commands {
		if command == nil {
			continue
		}
		if _, found := c.output[command.Name()]; found {
			return fmt.Errorf("unable to create Chain because cmd names are not unique (%s)", command.Name())
		}
		// assign a place holder
		c.output[command.Name()] = nil
	}
	return nil
}

func (c *Chain) runWithRetry(step int, command Runner) (out interface{}, err error) {
	c.step = step
	for try := 0; try < c.maxRetryOnError+1; try++ {
		out, err = command.Run()
		if err == nil {
			return out, err
		}
		time.Sleep(c.sleepBeforeRetry)
	}
	return out, err
}

func (c *Chain) rollbackWithRetry(command Runner) (err error) {
	for try := 0; try < c.maxRetryOnError+1; try++ {
		err = command.Rollback()
		if err == nil {
			return err
		}
		time.Sleep(c.sleepBeforeRetry)
	}
	return err
}
