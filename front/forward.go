package front

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fr-str/itsy-bitsy-teenie-weenie-port-forwarder-programini/config"
	"github.com/fr-str/itsy-bitsy-teenie-weenie-port-forwarder-programini/dns"
	"github.com/fr-str/itsy-bitsy-teenie-weenie-port-forwarder-programini/kube"
)

func (m model) forwardInputs() model {
	m.inputs = make([]textinput.Model, 2)
	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.CursorStyle = cursorStyle
		t.CharLimit = 5

		switch i {
		case 0:
			t.Placeholder = "ResourcePort"
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = "LocalPort"
		}

		m.inputs[i] = t
	}

	return m
}

func (m model) setupForward() (tea.Model, tea.Cmd) {

	if m.inputs[0].Value() == "" || m.inputs[1].Value() == "" {
		log.Info()
		return m.fpError("One of ports was empty")
	}
	pp, err := strconv.Atoi((m.inputs[0].Value()))

	if err != nil {
		return m.fpError(err.Error())
	}

	lp, err := strconv.Atoi((m.inputs[1].Value()))
	if err != nil {
		return m.fpError(err.Error())
	}

	// check port is already forwarded
	err = m.checkPorts(pp)
	if err != nil {
		return m.fpError(err.Error())
	}

	err = checkLocalPort(strconv.Itoa(lp))
	if err != nil {
		return m.fpError(err.Error())
	}

	var pf *kube.PortForwardA
	switch m.view {
	case podForwardView:
		pf = &kube.PortForwardA{
			Name:      m.selectedPod.Name,
			Namespace: m.selectedPod.Namespace,
			Resource:  "pods",
			KubePort:  pp,
			LocalPort: lp,
			OwnerName: m.selectedPod.OwnerName,
			Notify:    m.notify,
		}
		m.selectedPod.PFs = append(m.selectedPod.PFs, pf)

		// DNS
		ip := strings.ReplaceAll(m.selectedPod.IP, ".", "-")
		dns.Register(fmt.Sprintf(config.DNS_POD_FMT, ip, pf.Namespace), "127.0.0.1")
		dns.Register(fmt.Sprintf(config.DNS_POD_FMT+"cluster.local.", ip, pf.Namespace), "127.0.0.1")

	case serviceForwardView:
		pf = &kube.PortForwardA{
			Name:        m.selectedService.Name,
			ServiceName: m.selectedService.Name,
			Namespace:   m.selectedService.Namespace,
			Resource:    "services",
			KubePort:    pp,
			LocalPort:   lp,
			Notify:      m.notify,
		}
		m.selectedService.PFs = append(m.selectedService.PFs, pf)

		// DNS
		dns.Register(fmt.Sprintf(config.DNS_SERVICE_FMT, pf.Name, pf.Namespace), "127.0.0.1")
		dns.Register(fmt.Sprintf(config.DNS_SERVICE_FMT+"cluster.local.", pf.Name, pf.Namespace), "127.0.0.1")
	}

	go pf.Forward()

	m.view = m.lastView
	m.forwardError = ""
	m.notify <- kube.MapUpdateMsg{}
	return m.render()
}

func (m model) checkPorts(pp int) error {
	switch m.view {
	case podForwardView:
		for _, pf := range m.selectedPod.PFs {
			if pf.KubePort == pp {
				return fmt.Errorf("port already used")
			}
		}
	case serviceForwardView:
		for _, pf := range m.selectedService.PFs {
			if pf.KubePort == pp {
				return fmt.Errorf("port already used")
			}
		}
	}

	return nil
}

func checkLocalPort(lp string) error {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", lp), timeout)
	if err != nil {
		// Connection refused
		return nil
	}

	if conn != nil {
		defer conn.Close()
		return fmt.Errorf("localPort is used")
	}
	return nil
}

func (m model) fpError(msg string) (tea.Model, tea.Cmd) {

	m.forwardError = errColour + msg
	return m.render()
}

func testConnections() {
	for range time.Tick(time.Second) {
		for element := range kube.Map.Iter() {
			for pod := range element.Value.Iter() {
				for _, pf := range pod.Value.PFs {
					if pf != nil {
						go ping(pf)
					}
				}
			}
		}
	}
}

func ping(p *kube.PortForwardA) {
	if p == nil {
		return
	}
	r, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", p.LocalPort))
	if err != nil {
		log.Info(err)
		p.Condition = false
		return
	}
	r.Body.Close()
	p.Condition = true
}
