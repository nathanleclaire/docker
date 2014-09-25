package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/hosts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/reexec"
	"github.com/docker/docker/utils"
)

const (
	defaultCaFile   = "ca.pem"
	defaultKeyFile  = "key.pem"
	defaultCertFile = "cert.pem"
)

func main() {
	if reexec.Init() {
		return
	}
	flag.Parse()
	// FIXME: validate daemon flags here

	if *flVersion {
		showVersion()
		return
	}
	if *flDebug {
		os.Setenv("DEBUG", "1")
	}

	store := hosts.NewStore()

	if len(flHosts) == 0 {
		defaultHost := os.Getenv("DOCKER_HOST")
		// If the client, attempt to get active host
		if defaultHost == "" && !*flDaemon {
			host, err := store.GetActive()
			if err != nil {
				log.Fatal(err)
			}
			if host != nil {
				defaultHost, err = host.Driver.GetURL()
				if err != nil {
					log.Fatal(err)
				}
			}
		}
		if defaultHost == "" || *flDaemon {
			// If we do not have a host, default to unix socket
			defaultHost = fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET)
		}
		defaultHost, err := api.ValidateHostURL(defaultHost)
		if err != nil {
			log.Fatal(err)
		}
		flHosts = append(flHosts, defaultHost)
	}

	if *flDaemon {
		mainDaemon()
		return
	}

	if len(flHosts) > 1 {
		log.Fatal("Please specify only one -H")
	}

	hostURL := flHosts[0]

	// Attempt to find a host if it's a valid name
	if _, err := hosts.ValidateHostName(hostURL); err == nil {
		exists, err := store.Exists(hostURL)
		if err != nil {
			log.Fatal(err)
		}
		if !exists {
			log.Fatal(fmt.Errorf("Host %q does not exist. Create it using 'docker hosts create'.", hostURL))
		}
		host, err := store.Load(hostURL)
		if err != nil {
			log.Fatal(err)
		}
		hostURL, err = host.Driver.GetURL()
		if err != nil {
			log.Fatal(err)
		}
	}

	protoAddrParts := strings.SplitN(hostURL, "://", 2)

	var (
		cli       *client.DockerCli
		tlsConfig tls.Config
	)
	tlsConfig.InsecureSkipVerify = true

	// If we should verify the server, we need to load a trusted ca
	if *flTlsVerify {
		*flTls = true
		certPool := x509.NewCertPool()
		file, err := ioutil.ReadFile(*flCa)
		if err != nil {
			log.Fatalf("Couldn't read ca cert %s: %s", *flCa, err)
		}
		certPool.AppendCertsFromPEM(file)
		tlsConfig.RootCAs = certPool
		tlsConfig.InsecureSkipVerify = false
	}

	// If tls is enabled, try to load and send client certificates
	if *flTls || *flTlsVerify {
		_, errCert := os.Stat(*flCert)
		_, errKey := os.Stat(*flKey)
		if errCert == nil && errKey == nil {
			*flTls = true
			cert, err := tls.LoadX509KeyPair(*flCert, *flKey)
			if err != nil {
				log.Fatalf("Couldn't load X509 key pair: %s. Key encrypted?", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	if *flTls || *flTlsVerify {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], &tlsConfig)
	} else {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], nil)
	}

	if err := cli.Cmd(flag.Args()...); err != nil {
		if sterr, ok := err.(*utils.StatusError); ok {
			if sterr.Status != "" {
				log.Println(sterr.Status)
			}
			os.Exit(sterr.StatusCode)
		}
		log.Fatal(err)
	}
}

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.VERSION, dockerversion.GITCOMMIT)
}
