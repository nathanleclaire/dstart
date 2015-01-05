```
 _| __|_ _ .__|_
(_|_\ | (_||  | 
```

![dstart](https://cloud.githubusercontent.com/assets/1476820/5608618/d936079e-943e-11e4-858b-50e147132242.jpg)

Want to restart your Docker daemon but not nuke all of the containers you have running?  Always forget to run them with `--restart` or don't necessarily want to?  Then `dstart` may be for you!

`dstart` will stop all of the containers you have running, restart the Docker daemon, then restart all of the containers in the correct order- so your links are preserved!

To top it all off, it does all of this concurrently - so, if a container does not depend on other containers to start, the message to start it will be sent at the same time as all the other, unrelated ones.

Usage
=====

`dstart` is written in Go with just a few dependencies.

I will probably cut a binary soon, but if you want install it from source:

```console
$ go get -u github.com/Sirupsen/logrus
$ go get -u github.com/samalba/dockerclient
$ go get -u github.com/nathanleclaire/dstart
```

Now you have the `dstart` binary installed and you are just a `dstart` away from restart-ey goodness.
