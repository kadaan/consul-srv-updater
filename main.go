package main

import (
    "os"
    "log"

    "github.com/armon/consul-api"
    flags "github.com/jessevdk/go-flags"
    
    "github.com/mitchellh/goamz/aws"
)

type Options struct {
    DataDir string `short:"d" long:"data-dir" required:"true" description:"data directory"`
    ZoneId  string `short:"z" long:"zone"     required:"true" description:"route53 zone id"`
    Name    string `short:"n" long:"name"     required:"true" description:"SRV record name"`
    TTL     int    `short:"t" long:"ttl"      required:"true" description:"TTL"`
}

func main() {
    var opts Options
    
    _, err := flags.Parse(&opts)
    if err != nil {
        os.Exit(1)
    }

    consul, err := consulapi.NewClient(consulapi.DefaultConfig())
    
    if err != nil {
        panic(err)
    }
    
    awsAuth, err := aws.EnvAuth()

    if err != nil {
        panic(err)
    }
    
    wrapper := NewLockWrapper(consul, opts.DataDir)
    updater := NewSrvUpdater(awsAuth, opts.ZoneId)
    
    if ! wrapper.loadSession() || ! wrapper.isSessionValid() {
        wrapper.createSession()
    }

    if wrapper.acquireLock() || wrapper.haveLock() {
        log.Print("can do some stuff")
        
        // retrieve the list of consul servers
        services, _, err := consul.Catalog().Service("consul", "", nil)
        
        if err != nil {
            panic(err)
        }
        
        srvRecord := SrvRecord{
            Name: opts.Name,
            TTL:  opts.TTL,
            Targets: make([]SrvTarget, len(services)),
        }
        
        for ind, value := range services {
            // warning opinionated: Node name should be resolvable.  According
            // to the spec it should also be an A or AAAA record…
            
            // value.ServicePort returns the server's RPC address (8300 by
            // default), but joining the cluster requires using the server's
            // serf_lan port (8301 by default).  The serf_lan port isn't exposed
            // via the catalog, however.  Guess this'll just be more
            // opinionated, for now.
            srvRecord.Targets[ind] = SrvTarget{
                Priority: 10,
                Weight:   10,
                Port:     8301, // see above
                Target:   value.Node,
            }
        }
        
        err = updater.UpdateRecord(&srvRecord)
        
        if err != nil {
            log.Fatal("unable to update record: ", err)
        }
    } else {
        log.Print("unable to lock key")
    }
}
