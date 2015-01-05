package main

import (
	"os/exec"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/samalba/dockerclient"
)

var (
	// TODO: should this be shared struct property?
	docker dockerclient.Client
)

func getNamesFromRawLinks(rawLinks []string) ([]string, error) {
	names := []string{}
	for _, rawLinks := range rawLinks {
		// ORIGINAL : "/redis:/adoring_stallman/redis"
		// DESIRED  : "redis" (the last bit of the string)
		// TODO: can this be simplified?
		name := strings.Split(strings.Split(rawLinks, ":")[1], "/")[2]
		names = append(names, name)
	}
	return names, nil
}

func getLinks(info dockerclient.ContainerInfo) ([]string, error) {
	rawLinks := info.HostConfig.Links
	links, err := getNamesFromRawLinks(rawLinks)
	if err != nil {
		return []string{}, err
	}
	log.Debugln("links for", info.Name, "are", links)
	return links, nil
}

func waitForDeps(deps []string, rollers <-chan string) {
	depRemaining := make(map[string]bool)
	for _, link := range deps {
		depRemaining[link] = true
	}
	for lastRestarted := range rollers {
		depRemaining[lastRestarted] = false
		log.Debugln("depRemaining is", depRemaining)
		for _, depRemains := range depRemaining {
			if depRemains {
				continue
			}
		}
		return
	}
}

func pollRestart(c dockerclient.Container, done chan<- string, rollers <-chan string) {
	log.Infoln("Initiating restart for ", c.Id)

	info, err := docker.InspectContainer(c.Id)
	if err != nil {
		log.Fatal(err)
	}

	deps, err := getLinks(*info)
	if err != nil {
		log.Fatal(err)
	}

	if len(deps) > 0 {
		waitForDeps(deps, rollers)
	}

	log.Debugln("Starting", c.Id)

	// Alright, all the deps are done restarting,
	// so let's rock and roll!
	if err := docker.StartContainer(c.Id, nil); err != nil {
		log.Fatal(err)
	}

	// lose the "/" on the beginning of container name
	done <- info.Name[1:]

	// Just catch the rest until the channel closes
	for lastRestarted := range rollers {
		log.Debugln("Read", lastRestarted, "from rollers in goroutine for", c.Id[:7])
	}
}

func main() {
	var (
		err error
	)
	start := time.Now()
	docker, err = dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		log.Fatal(err)
	}

	restartDone := make(chan string)

	// todo: should this be slice?
	// i.e. is there any reason to track the id in key?
	rMsgs := []chan string{}

	containers, err := docker.ListContainers(false, false, "")
	if err != nil {
		log.Fatal(err)
	}

	stopMsgs := make(chan bool)
	for _, c := range containers {
		rMsgs = append(rMsgs, make(chan string))
		go func(id string) {
			log.Infoln("Initiating stop for container", id)
			if err := docker.StopContainer(id, 30); err != nil {
				log.Fatal(err)
			}
			stopMsgs <- true
		}(c.Id)
	}

	// wait for all containers to be stopped
	for i := 0; i < len(containers); i++ {
		<-stopMsgs
	}
	close(stopMsgs)

	log.Info("All containers stopped")

	// restart the daemon
	log.Info("Restarting Docker...")
	if err := exec.Command("sudo", "service", "docker", "restart").Run(); err != nil {
		log.Fatal(err)
	}

	// TODO: Hack because the socket accepts
	// connections before the API is ready to
	// accept requests.
	for {
		_, err := docker.Info()
		if err != nil {
			log.Debug(err)
		} else {
			log.Info("Daemon restarted successfully")
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// initiate each container polling for its deps to
	// know when it can restart
	for i, c := range containers {
		go pollRestart(c, restartDone, rMsgs[i])
	}

	for i := 0; i < len(containers); i++ {
		r := <-restartDone
		log.Debugln("Read from channel that", r, "started")
		for _, rMsg := range rMsgs {
			go func(rMsg chan<- string, r string) {
				log.Debugln("SENDING", r)
				rMsg <- r
			}(rMsg, r)
		}
	}

	close(restartDone)
	for _, rMsg := range rMsgs {
		close(rMsg)
	}

	log.Infof("Restarted the daemon and %d containers in %s", len(containers), time.Since(start))
}
