package env

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"hanzo.io/util/log"
)

// Lifted from https://github.com/golang/appengine/blob/master/internal/base/api_base.pb.go
type StringProto struct {
	Value            *string `protobuf:"bytes,1,req,name=value" json:"value,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *StringProto) Reset()         { *m = StringProto{} }
func (m *StringProto) String() string { return proto.CompactTextString(m) }
func (*StringProto) ProtoMessage()    {}

func (m *StringProto) GetValue() string {
	if m != nil && m.Value != nil {
		return *m.Value
	}
	return ""
}

// Dev app server script filename
const AppServerFileName = "dev_appserver.py"
const AEFakeName = "appenginetestingfake"

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findDevAppserver() (string, error) {
	if p := os.Getenv("APPENGINE_DEV_APPSERVER"); p != "" {
		if fileExists(p) {
			return p, nil
		}
		return "", fmt.Errorf("invalid APPENGINE_DEV_APPSERVER environment variable; path %q doesn't exist", p)
	}
	return exec.LookPath(AppServerFileName)
}

func findPython() (path string, err error) {
	for _, name := range []string{"python2.7", "python"} {
		path, err = exec.LookPath(name)
		if err == nil {
			return
		}
	}
	return
}

func Call(c context.Context, service, method string, in, out proto.Message) error {
	if service == "__go__" {
		currentNamespace := proto.String(c.Value("req").(*http.Request).Header.Get("X-AppEngine-Current-Namespace"))
		defaultNamespace := proto.String(c.Value("req").(*http.Request).Header.Get("X-AppEngine-Default-Namespace"))

		if method == "GetNamespace" {
			out.(*StringProto).Value = currentNamespace
			return nil
		}

		if method == "GetDefaultNamespace" {
			out.(*StringProto).Value = defaultNamespace
			return nil
		}
	}

	// DAVID STOP CARGO CULT PROGRAMMING
	// cn := internal.NamespaceFromContext(c)
	// if cn != "" {
	// 	if mod, ok := internal.NamespaceMods[service]; ok {
	// 		mod(in, cn)
	// 	}
	// }

	data, err := proto.Marshal(in)
	if err != nil {
		return err
	}

	req, _ := http.NewRequest("POST",
		fmt.Sprintf("%s/call?s=%s&m=%s", c.Value("testingURL").(string), service, method),
		bytes.NewBuffer(data))

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		body, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("got status %d; body: %q", res.StatusCode, body)
	}

	pbytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	return proto.Unmarshal(pbytes, out)
}

func StartChild(c *FancyContext) error {
	python, err := findPython()
	if err != nil {
		return fmt.Errorf("Could not find python interpreter: %v", err)
	}

	c.FakeAppDir, err = ioutil.TempDir("", AEFakeName)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			// cleanup directory if there's an error in any of the steps following the creation of the child
			fmt.Printf("Cleaning up directory because of an error - %v\n", err)
			os.RemoveAll(c.FakeAppDir)
		}
	}()

	appBuf := new(bytes.Buffer)
	appTempl.Execute(appBuf, c.AppId)
	err = ioutil.WriteFile(filepath.Join(c.FakeAppDir, AEFakeName+".yaml"), appBuf.Bytes(), 0755)
	if err != nil {
		return err
	}

	c.Modules = append(c.Modules, ModuleConfig{Name: AEFakeName, Path: filepath.Join(c.FakeAppDir, AEFakeName+".yaml")})

	if len(c.Queues) > 0 {
		var queueBuf bytes.Buffer
		queueTempl.Execute(&queueBuf, c.Queues)
		err = ioutil.WriteFile(filepath.Join(c.FakeAppDir, "queue.yaml"), queueBuf.Bytes(), 0755)
		if err != nil {
			return fmt.Errorf("Error generating queue.yaml - %v", err)
		}
	}

	var helperBuf bytes.Buffer
	helperTempl.Execute(&helperBuf, AEFakeName)
	err = ioutil.WriteFile(filepath.Join(c.FakeAppDir, AEFakeName+".go"), helperBuf.Bytes(), 0644)
	if err != nil {
		return err
	}

	devAppserver, err := findDevAppserver()
	if err != nil {
		return err
	}

	startupComponents := []ComponentURL{
		ComponentURL{Name: "appenginetestingapi", Regex: regexp.MustCompile(`Starting API server at: (\S+)`)},
		ComponentURL{Name: "appenginetestingadmin", Regex: regexp.MustCompile(`Starting admin server at: (\S+)`)},
	}

	params := []string{}
	for _, val := range c.Modules {
		startupComponents = append(startupComponents,
			ComponentURL{
				Name:  val.Name,
				Regex: regexp.MustCompile(fmt.Sprintf(`Starting module "%s" running at: (\S+)`, val.Name)),
			})
		params = append(params, val.Path)
	}

	appLog := LogChild
	if c.Debug == LogChild {
		appLog = LogDebug
	}

	appLog = LogDebug

	args := append([]string{
		devAppserver,
		"--clear_datastore=true",
		"--datastore_consistency_policy=consistent",
		"--skip_sdk_update_check=true",
		fmt.Sprintf("--storage_path=%s", c.FakeAppDir),
		fmt.Sprintf("--log_level=%s", appLog),
		"--dev_appserver_log_level=debug",
		"--port=0",
		"--api_port=0",
		"--admin_port=0",
	}, params...)

	switch runtime.GOOS {
	case "windows":
		c.Child = exec.Command("cmd", append([]string{"/C", python}, args...)...)
	case "linux":
		fallthrough
	case "darwin":
		c.Child = exec.Command(python, args...)
	default:
		err = fmt.Errorf("env not supported on your platform of %s", runtime.GOOS)
		return err
	}

	c.Child.Stdout = os.Stdout
	var stderr io.Reader
	stderr, err = c.Child.StderrPipe()
	if err != nil {
		return err
	}

	if err = c.Child.Start(); err != nil {
		return err
	}

	// Wait until we have read the URL of all startup components
	errc := make(chan error, 1)
	componentsc := make(chan ComponentURL)
	startupComponentsCopy := make([]ComponentURL, len(startupComponents))
	copy(startupComponentsCopy, startupComponents)
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			if c.Debug == LogChild {
				log.Info("%s", s.Text())
			}
			for _, componentURL := range startupComponentsCopy {
				if match := componentURL.Regex.FindSubmatch(s.Bytes()); match != nil {
					componentURL.URL = string(match[1])
					componentsc <- componentURL
				}
			}
		}
		if err := s.Err(); err != nil {
			errc <- err
		}
	}()

	for {
		allStarted := true
		for _, cu := range startupComponents {
			if cu.URL == "" {
				allStarted = false
				break
			}
		}

		if allStarted {
			return nil
		}

		select {
		case compURL := <-componentsc:
			if compURL.Name == AEFakeName {
				c.TestingURL = compURL.URL
			}
			for x, value := range startupComponents {
				if value.Name == compURL.Name {
					startupComponents[x] = compURL
					break
				}
			}
		case <-time.After(60 * time.Second):
			if p := c.Child.Process; p != nil {
				p.Kill()
			}
			Close(c)
			for _, value := range startupComponents {
				if value.URL == "" {
					for _, m := range c.Modules {
						if m.Name == value.Name {
							return fmt.Errorf("timeout starting child process supporting - %s, does %s contain module config named %s?", m.Name, m.Path, m.Name)
						}
					}
					return fmt.Errorf("timeout starting child process supporting - %s", value.Name)
				}
			}
			return errors.New("Timeout starting process, this error is a bug in appenginetesting")
		case err = <-errc:
			Close(c)
			return fmt.Errorf("error reading child process stderr: %v", err)
		}
	}
}
