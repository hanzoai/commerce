package ginkgo

import (
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"crowdstart.io/util/log"
)

func Setup(suiteName string, t *testing.T) {
	log.SetVerbose(testing.Verbose())
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, suiteName)
}
