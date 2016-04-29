package main

import (
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var wg sync.WaitGroup

func terminated() {
	func() {
		defer func() {
			recover()
		}()
		wg.Done()
	}()
}

// stop specified proc.
func stopProc(proc string, quit bool) error {
	p, ok := procs[proc]
	if !ok {
		return errors.New("Unknown proc: " + proc)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil {
		return nil
	}

	p.quit = quit
	err := terminateProc(proc)
	if err != nil {
		return err
	}
	timeout := time.AfterFunc(10*time.Second, func() {
		if p, ok := procs[proc]; ok {
			err = p.cmd.Process.Kill()
		}
	})
	p.cond.Wait()
	timeout.Stop()
	err = p.waitErr
	if err == nil {
		p.cmd = nil
	} else if p.cmd != nil && p.cmd.Process != nil {
		err = p.cmd.Process.Kill()
	}
	return err
}

// start specified proc. if proc is started already, return nil.
func startProc(proc string) error {
	p, ok := procs[proc]
	if !ok {
		return errors.New("Unknown proc: " + proc)
	}

	p.mu.Lock()
	if procs[proc].cmd != nil {
		p.mu.Unlock()
		return nil
	}

	go func() {
		if spawnProc(proc) {
			terminated()
		}
		p.mu.Unlock()
	}()
	return nil
}

// restart specified proc.
func restartProc(proc string) error {
	if _, ok := procs[proc]; !ok {
		return errors.New("Unknown proc: " + proc)
	}
	stopProc(proc, false)
	return startProc(proc)
}

// spawn all procs.
func startProcs() error {
	wg.Add(len(procs))
	for proc := range procs {
		startProc(proc)
	}
	sc := make(chan os.Signal, 10)
	go func() {
		wg.Wait()
		sc <- syscall.SIGINT
	}()
	signal.Notify(sc, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	<-sc
	for proc := range procs {
		stopProc(proc, true)
	}
	return nil
}
