package main

import (
	"os"

	"github.com/oasdiff/go-common/ds"
	"github.com/oasdiff/go-common/env"
	"github.com/oasdiff/go-common/gcs"
	"github.com/oasdiff/refresh/internal"
)

func main() {

	dsc := ds.NewClientWrapper(env.GetGCloudProject())
	defer dsc.Close()

	store := gcs.NewStore()
	defer store.Close()

	if err := internal.Run(dsc, store); err != nil {
		os.Exit(1)
	}
}
