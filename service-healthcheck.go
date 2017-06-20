package main

import (
	"fmt"
	"time"
	"strings"
	"bytes"
	"os/exec"

	"github.com/parnurzeal/gorequest"
	"gopkg.in/alecthomas/kingpin.v2"
	log "github.com/Sirupsen/logrus"
)

const (
	ver string = "0.10"
	restartMaxBackOffDelaySeconds = 1000
)

var (
	url = kingpin.Flag("url", "health check URL").Default("http://127.0.0.1:8080").Short('u').String()
	service = kingpin.Flag("service", "local service name").Short('s').Required().String()
	timeout = kingpin.Flag("timeout", "timeout for HTTP requests in seconds").Default("3").Short('t').Int()
	interval = kingpin.Flag("interval", "healthcheck interval").Default("5").Short('i').Int()
	failInterval = kingpin.Flag("fail-interval", "healthcheck interval during failed period").Default("1").Int()
	failThreshold = kingpin.Flag("fail-threshold", "failed healthchecks threshold").Default("3").Int()
	dryRun = kingpin.Flag("dry-run", "dry run").Short('n').Bool()
)

type Msg struct {
	response string
	err error
}

func httpGet(url string, ch chan Msg) {
	var msg Msg
	request := gorequest.New()
	resp, body, errs := request.Get(url).End()

	if errs != nil {
		var errsStr []string
		for _, e := range errs {
			errsStr = append(errsStr, fmt.Sprintf("%s", e))
		}
		msg.err = fmt.Errorf("%s", strings.Join(errsStr, ", "))
		ch <- msg
		return
	}
	if resp.StatusCode == 200 {
		msg.response = body
	} else {
		msg.err = fmt.Errorf("HTTP response code: %s", resp.Status)
	}
	ch <- msg
}

func restartUpstartService(service string) error {
	command := "service"
	args := []string{service, "restart"}
	cmd := exec.Command(command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf(fmt.Sprint(err) + ": " + stderr.String())
	}
	return nil
}

func sleepLoop(failedHealthchecks chan bool, interval, failInterval int) {
	if len(failedHealthchecks) > 0 {
		time.Sleep(time.Second * time.Duration(failInterval))
	} else {
		time.Sleep(time.Second * time.Duration(interval))
	}
}

func emptyChannel(ch chan bool) {
	Loop:
		for {
			select {
			case <-ch:
			default:
				break Loop
			}
		}
}

func setHealthcheckSuccess(failedHealthchecks, restartRequired, restartCounter chan bool) {
	emptyChannel(failedHealthchecks)
	emptyChannel(restartRequired)
	emptyChannel(restartCounter)
}

func setHealthcheckFailed(failedHealthchecks, restartRequired chan bool, failThreshold int) {
	if len(failedHealthchecks) < failThreshold {
		failedHealthchecks <- true
	}
	if len(restartRequired) == 0 {
		restartRequired <- true
	}
}

func serviceHealthcheck(failedHealthchecks, restartRequired, restartCounter chan bool, url string, timeout, interval, failInterval, failThreshold int) {
	for {
		ch := make(chan Msg)
		go httpGet(url, ch)

		var msg Msg
		select {
		case msg = <-ch:
			// healthcheck success
			if msg.err == nil {
				if len(failedHealthchecks) > 0 {
					log.Infof("Healthcheck recovered")
					setHealthcheckSuccess(failedHealthchecks, restartRequired, restartCounter)
				}
			// healthcheck failed
			} else {
				log.Infof("Healthcheck failed: %s", msg.err)
				setHealthcheckFailed(failedHealthchecks, restartRequired, failThreshold)
			}
		// timeout
		case <-time.After(time.Second * time.Duration(timeout)):	
			log.Info("Healthcheck timeout")
			setHealthcheckFailed(failedHealthchecks, restartRequired, failThreshold)
		}

		sleepLoop(failedHealthchecks, interval, failInterval)
	}
}

func incrementCounter(restartCounter chan bool) {
	if len(restartCounter) < restartMaxBackOffDelaySeconds {
		restartCounter <- true
	}
}

func serviceRestart(failedHealthchecks, restartRequired, restartCounter chan bool, service string, dryRun bool, failThreshold int) {
	for {
		select {
		case <-restartRequired:
			if len(failedHealthchecks) >= failThreshold {
				log.Infof("Restarting service %s", service)
				if dryRun {
					log.Info("Dry run, skipping")
				} else {
					// restart success
					if err := restartUpstartService(service); err == nil {
						log.Infof("Service %s restarted", service)
					// restart failed
					} else {
						log.Errorf("Cannot restart service %s: %s", service, err)
					}
					incrementCounter(restartCounter)
				}
			}
		}
		time.Sleep(time.Second * time.Duration(len(restartCounter)))
	}
}

func main() {
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	log.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	kingpin.Version(ver)
	kingpin.Parse()

	if *dryRun {
		log.Info("Running in dry run mode")
	}

	failedHealthchecks := make(chan bool, *failThreshold + 1)
	restartRequired := make(chan bool, 2)
	restartCounter := make(chan bool, restartMaxBackOffDelaySeconds + 1)
	go serviceHealthcheck(failedHealthchecks, restartRequired, restartCounter, *url, *timeout, *interval, *failInterval, *failThreshold)
	go serviceRestart(failedHealthchecks, restartRequired, restartCounter, *service, *dryRun, *failThreshold)
	select {}
}
