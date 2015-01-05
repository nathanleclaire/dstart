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
		// DESIRED  : "adoring_stallman"
		name := strings.Split(strings.Split(rawLinks, ":")[1], "/")[2]
		names = append(names, name)
	}
	return names, nil
}

func getLinks(c dockerclient.Container) ([]string, error) {
	info, err := docker.InspectContainer(c.Id)
	if err != nil {
		return []string{}, err
	}
	rawLinks := info.HostConfig.Links
	links, err := getNamesFromRawLinks(rawLinks)
	if err != nil {
		return []string{}, err
	}
	log.Infoln("links for", info.Name, "are", links)
	return links, nil
}

func pollRestart(c dockerclient.Container, done chan string, rollers chan string) {
	log.Infoln("Initiating poll restart for ", c.Id)
	depRemaining := make(map[string]bool)
	deps, err := getLinks(c)
	if err != nil {
		log.Fatal(err)
	}
	for _, link := range deps {
		depRemaining[link] = true
	}

	if len(deps) > 0 {
	DepCheck:
		for lastRestarted := range rollers {
			log.Warnln("Read", lastRestarted, "from rollers in goroutine for", c.Id[:7])
			depRemaining[lastRestarted] = false
			log.Infoln("depRemaining is", depRemaining)
			for _, depRemains := range depRemaining {
				if depRemains {
					continue DepCheck
				}
			}
			break DepCheck
		}
	}

	// TODO: should we just scrap getLinks() function
	// altogether if I need to inline this anyway?
	info, err := docker.InspectContainer(c.Id)
	if err != nil {
		log.Fatal(err)
	}

	log.Warnln("Starting", c.Id)

	// Alright, all the deps are done restarting,
	// so let's rock and roll!
	if err := docker.StartContainer(c.Id, nil); err != nil {
		log.Fatal(err)
	}

	// lose the "/" on the beginning of container name
	done <- info.Name[1:]

	// Just catch the rest until the channel closes
	for lastRestarted := range rollers {
		log.Warnln("Read", lastRestarted, "from rollers in goroutine for", c.Id[:7])
	}
}

func main() {
	var (
		err error
	)
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
		log.Infoln(c.Id, c.Names)
		rMsgs = append(rMsgs, make(chan string))
		go func(id string) {
			log.Infoln("Stopping container", id)
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

	// TODO: this actually works
	// to prevent the race condition ಠ_ಠ
	// use something other than a sleep.
	time.Sleep(5 * time.Second)

	// initiate each container polling for its deps to
	// know when it can restart
	for i, c := range containers {
		go pollRestart(c, restartDone, rMsgs[i])
	}

	for i := 0; i < len(containers); i++ {
		r := <-restartDone
		log.Infoln("Read from channel that", r, "started")
		for _, rMsg := range rMsgs {
			go func(rMsg chan string, r string) {
				log.Warnln("SENDING", r)
				// TODO: better way to implement this?
				rMsg <- r
			}(rMsg, r)
		}
	}

	close(restartDone)
	for _, rMsg := range rMsgs {
		close(rMsg)
	}
	log.Infof("Restarted %d containers", len(containers))
}
