package ginkgo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"

	"hanzo.io/log"
	"hanzo.io/util/retry"
)

func Setup(suiteName string, t *testing.T) {
	// log.Error("testing.Verbose", testing.Verbose())
	log.SetVerbose(testing.Verbose())
	gomega.RegisterFailHandler(ginkgo.Fail)

	// Write XML if on CI
	xmldir := os.Getenv("CIRCLE_TEST_REPORTS")
	if xmldir == "" {
		ginkgo.RunSpecs(t, suiteName)
	} else {
		junitReporter := reporters.NewJUnitReporter(filepath.Join(xmldir, "junit.xml"))
		ginkgo.RunSpecsWithDefaultAndCustomReporters(t, suiteName, []ginkgo.Reporter{junitReporter})
	}
}

var Retry = retry.Retry

// Declarations for Ginkgo DSL
type Done ginkgo.Done
type Benchmarker ginkgo.Benchmarker

var GinkgoWriter = ginkgo.GinkgoWriter
var GinkgoParallelNode = ginkgo.GinkgoParallelNode
var GinkgoT = ginkgo.GinkgoT
var CurrentGinkgoTestDescription = ginkgo.CurrentGinkgoTestDescription
var RunSpecs = ginkgo.RunSpecs
var RunSpecsWithDefaultAndCustomReporters = ginkgo.RunSpecsWithDefaultAndCustomReporters
var RunSpecsWithCustomReporters = ginkgo.RunSpecsWithCustomReporters
var Fail = ginkgo.Fail
var GinkgoRecover = ginkgo.GinkgoRecover
var Describe = ginkgo.Describe
var FDescribe = ginkgo.FDescribe
var PDescribe = ginkgo.PDescribe
var XDescribe = ginkgo.XDescribe
var Context = ginkgo.Context
var FContext = ginkgo.FContext
var PContext = ginkgo.PContext
var XContext = ginkgo.XContext
var It = ginkgo.It
var FIt = ginkgo.FIt
var PIt = ginkgo.PIt
var XIt = ginkgo.XIt
var By = ginkgo.By
var Measure = ginkgo.Measure
var FMeasure = ginkgo.FMeasure
var PMeasure = ginkgo.PMeasure
var XMeasure = ginkgo.XMeasure

var BeforeSuite = ginkgo.BeforeSuite
var AfterSuite = ginkgo.AfterSuite
var SynchronizedBeforeSuite = ginkgo.SynchronizedBeforeSuite
var SynchronizedAfterSuite = ginkgo.SynchronizedAfterSuite

var Before = ginkgo.BeforeEach
var After = ginkgo.AfterEach
var JustBefore = ginkgo.JustBeforeEach

var BeforeEach = ginkgo.BeforeEach
var AfterEach = ginkgo.AfterEach
var JustBeforeEach = ginkgo.JustBeforeEach

var Skip = ginkgo.Skip

// Declarations for Gomega DSL
var RegisterFailHandler = gomega.RegisterFailHandler
var RegisterTestingT = gomega.RegisterTestingT
var InterceptGomegaFailures = gomega.InterceptGomegaFailures
var Ω = gomega.Ω
var Expect = gomega.Expect
var ExpectWithOffset = gomega.ExpectWithOffset
var Eventually = gomega.Eventually
var EventuallyWithOffset = gomega.EventuallyWithOffset
var Consistently = gomega.Consistently
var ConsistentlyWithOffset = gomega.ConsistentlyWithOffset
var SetDefaultEventuallyTimeout = gomega.SetDefaultEventuallyTimeout
var SetDefaultEventuallyPollingInterval = gomega.SetDefaultEventuallyPollingInterval
var SetDefaultConsistentlyDuration = gomega.SetDefaultConsistentlyDuration
var SetDefaultConsistentlyPollingInterval = gomega.SetDefaultConsistentlyPollingInterval

// Declarations for Gomega Matchers
var Equal = gomega.Equal
var BeEquivalentTo = gomega.BeEquivalentTo
var BeNil = gomega.BeNil
var BeTrue = gomega.BeTrue
var BeFalse = gomega.BeFalse
var HaveOccurred = gomega.HaveOccurred
var Succeed = gomega.Succeed
var MatchError = gomega.MatchError
var BeClosed = gomega.BeClosed
var Receive = gomega.Receive
var BeSent = gomega.BeSent
var MatchRegexp = gomega.MatchRegexp
var ContainSubstring = gomega.ContainSubstring
var HavePrefix = gomega.HavePrefix
var HaveSuffix = gomega.HaveSuffix
var MatchJSON = gomega.MatchJSON
var BeEmpty = gomega.BeEmpty
var HaveLen = gomega.HaveLen
var BeZero = gomega.BeZero
var ContainElement = gomega.ContainElement
var ConsistOf = gomega.ConsistOf
var HaveKey = gomega.HaveKey
var HaveKeyWithValue = gomega.HaveKeyWithValue
var BeNumerically = gomega.BeNumerically
var BeTemporally = gomega.BeTemporally
var BeAssignableToTypeOf = gomega.BeAssignableToTypeOf
var Panic = gomega.Panic

// Helpers for nested Expect calls
func Expect1(actual interface{}, extra ...interface{}) gomega.GomegaAssertion {
	return ExpectWithOffset(1, actual, extra...)
}

func Expect2(actual interface{}, extra ...interface{}) gomega.GomegaAssertion {
	return ExpectWithOffset(2, actual, extra...)
}

func Expect3(actual interface{}, extra ...interface{}) gomega.GomegaAssertion {
	return ExpectWithOffset(3, actual, extra...)
}

// BeforeAll / AfterAll helpers
func BeforeAll(fn func()) {
	first := true
	Before(func() {
		// Only run first time BeforeEach block is executed
		if first {
			fn()
			first = false
		}
	})
}

func AfterAll(fn func()) {
	first := true
	After(func() {
		// Only run first time AfterEach block is executed
		if first {
			fn()
			first = false
		}
	})
}
