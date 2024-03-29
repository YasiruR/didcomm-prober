package cli

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/YasiruR/didcomm-prober/didcomm/discovery"
	"github.com/YasiruR/didcomm-prober/domain"
	"github.com/YasiruR/didcomm-prober/domain/container"
	"github.com/YasiruR/didcomm-prober/domain/models"
	"github.com/YasiruR/didcomm-prober/domain/services"
	internalLog "github.com/YasiruR/didcomm-prober/log"
	"github.com/tryfix/log"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

type runner struct {
	cfg     *container.Config
	reader  *bufio.Reader
	prober  services.Agent
	disc    services.Discoverer
	log     log.Logger
	outChan chan string
	disCmds uint64 // flag to identify whether output cursor is on basic commands or not
	pubsub  services.GroupAgent
	verbose bool
}

func ParseArgs() *container.Args {
	n := flag.String(`label`, ``, `agent's name'`)
	p := flag.Int(`port`, 0, `agent's port'`)
	pub := flag.Int(`pub_port`, 0, `agent's publishing port'`)
	mocker := flag.Bool(`mock`, true, `enables mocking functions`)
	mockPort := flag.Int(`mock_port`, 0, `port for mocking functions`)
	v := flag.Bool(`v`, false, `logging`)
	flag.Parse()

	if *mocker == true && *mockPort == 0 {
		fmt.Println("mock server port should be provided when enabled (see -h or --help for details)")
		os.Exit(0)
	}

	return &container.Args{
		Name:     *n,
		Port:     *p,
		PubPort:  *pub,
		Mocker:   *mocker,
		MockPort: *mockPort,
		Verbose:  *v,
	}
}

func Init(c *container.Container) {
	fmt.Printf("-> Agent initialized with following attributes: \n\t- Name: %s\n\t- Hostname: %s\n", c.Cfg.Args.Name, c.Cfg.Hostname[:len(c.Cfg.Hostname)-1])
	fmt.Printf("-> Press c and enter for commands\n")

	r := &runner{
		cfg:     c.Cfg,
		reader:  bufio.NewReader(os.Stdin),
		prober:  c.Prober,
		disc:    discovery.NewDiscoverer(c),
		outChan: c.OutChan,
		log:     internalLog.NewLogger(c.Cfg.Verbose, 5, internalLog.LevelTrace),
		pubsub:  c.PubSub,
		verbose: c.Cfg.Verbose,
	}

	go r.listen()
	//r.basicCommands()
	r.enableCommands()
}

/* basic functions */

func (r *runner) basicCommands() {
basicCmds:
	fmt.Printf("\n-> Enter the corresponding number of a command to proceed;\n\t" +
		"[1] Generate invitation\n\t" +
		"[2] Connect via invitation\n\t" +
		"[3] Send a message\n\t" +
		"[4] Create a group\n\t" +
		"[5] Join a group\n\t" +
		"[6] Send group message\n\t" +
		"[7] Leave group\n\t" +
		"[8] Group Info\n\t" +
		"[9] Discover Features\n\t" +
		"[b] Back\n\t" +
		"[e] Exit\n   Command: ")
	atomic.AddUint64(&r.disCmds, 1)

	cmd, err := r.reader.ReadString('\n')
	if err != nil {
		fmt.Println("   Error: reading command number failed, please try again")
		goto basicCmds
	}

	switch strings.TrimSpace(cmd) {
	case "1":
		r.generateInvitation()
	case "2":
		r.connectWithInv()
	case "3":
		r.sendMsg()
	case "4":
		r.createGroup()
	case "5":
		r.joinGroup()
	case "6":
		r.groupMsg()
	case "7":
		r.leave()
	case "8":
		r.groupInfo()
	case "9":
		r.discover()
	case "b":

	case "e":
		fmt.Println(`program exited`)
		os.Exit(0)
	default:
		if r.disCmds > 0 {
			fmt.Println("   Error: invalid command number, please try again")
			goto basicCmds
		}
	}

	atomic.StoreUint64(&r.disCmds, 0)
	//r.basicCommands()
	r.enableCommands()
}

func (r *runner) enableCommands() {
	input, err := r.reader.ReadString('\n')
	if err != nil {
		fmt.Println("   Error: reading instruction failed, please try again")
	}

	if strings.TrimSpace(input) == `c` || strings.TrimSpace(input) == `C` {
		r.basicCommands()
	} else {
		r.enableCommands()
	}
}

func (r *runner) listen() {
	for {
		text := <-r.outChan
		if !r.verbose {
			continue
		}

		if r.disCmds == 1 {
			atomic.StoreUint64(&r.disCmds, 0)
			fmt.Println()
		}
		r.output(text, true)
	}
}

/* command specific functions */

func (r *runner) generateInvitation() {
	inv, err := r.prober.Invite()
	if err != nil {
		r.error(`generating invitation failed`, err)
		return
	}

	r.output(fmt.Sprintf("Invitation URL: %s", inv), true)
}

func (r *runner) connectWithInv() {
	u, err := url.Parse(r.input(`Provide invitation in URL form`))
	if err != nil {
		r.error(`invalid url format, please try again`, err)
		return
	}

	inv, ok := u.Query()[`oob`]
	if !ok {
		r.error(`invitation url must contain 'oob' parameter, please try again`, err)
		return
	}

	if _, err = r.prober.Accept(inv[0]); err != nil {
		r.error(`invitation may be invalid, please try again`, err)
	}
}

func (r *runner) sendMsg() {
	peer := r.input(`Recipient`)
	msg := r.input(`Message`)

	if err := r.prober.SendMessage(models.TypData, peer, msg); err != nil {
		r.error(`sending message failed`, err)
	}
}

func (r *runner) discover() {
	endpoint := r.input(`Endpoint`)
	query := r.input(`Query`)
	comment := r.input(`Comment`)
	features, err := r.disc.Query(endpoint, query, comment)
	if err != nil {
		r.error(`discovering features failed, please try again`, err)
		return
	}

	var list []string
	for _, f := range features {
		list = append(list, fmt.Sprintf(`Protocol: "%s", Roles: %v`, f.Id, f.Roles))
	}
	r.outputList(`Supported features`, list)
}

func (r *runner) createGroup() {
	topic := r.input(`Topic`)
	strPub := r.input(`Publisher (Y/N)`)
	mode := r.input(`Group queue mode [single,multiple] (S/M)`)
	strJoinConsist := r.input(`Strict consistency for join operation (Y/N)`)
	strOrdrd := r.input(`Causal consistency for group messages (Y/N)`)
	//mode, strJoinConsist, strOrdrd, strPub = `m`, `y`, `y`, `y`

	publisher, err := r.validBool(strPub)
	if err != nil {
		r.error(`invalid input`, err)
		return
	}

	ordrd, err := r.validBool(strOrdrd)
	if err != nil {
		r.error(`invalid input`, err)
		return
	}

	joinConsist, err := r.validBool(strJoinConsist)
	if err != nil {
		r.error(`invalid input`, err)
		return
	}

	var gm domain.GroupMode
	if mode == `s` || mode == `S` {
		gm = domain.SingleQueueMode
	} else {
		gm = domain.MultipleQueueMode
	}

	if err = r.pubsub.Create(topic, publisher,
		models.GroupParams{OrderEnabled: ordrd, JoinConsistent: joinConsist, Mode: gm},
	); err != nil {
		r.error(`create group failed`, err)
		return
	}
	r.output(`Group created`, true)
}

func (r *runner) joinGroup() {
	topic := r.input(`Topic`)
	acceptor := r.input(`Acceptor`)
	strPub := r.input(`Publisher (Y/N)`)

	publisher, err := r.validBool(strPub)
	if err != nil {
		r.error(`invalid input`, err)
		return
	}

	if err = r.pubsub.Join(topic, acceptor, publisher); err != nil {
		r.error(`group join failed`, err)
		return
	}
	r.output(`Joined group `+topic, true)
}

func (r *runner) groupMsg() {
	topic := r.input(`Topic`)
	msg := r.input(`Message`)

	if _, err := r.pubsub.Send(topic, msg); err != nil {
		r.error(`sending group message failed`, err)
	}
}

func (r *runner) leave() {
	topic := r.input(`Topic`)
	if err := r.pubsub.Leave(topic); err != nil {
		r.error(`leaving group failed`, err)
	}
}

func (r *runner) groupInfo() {
	topic := r.input(`Topic`)
	params, mems := r.pubsub.Info(topic)
	if mems == nil {
		r.output(`Topic does not exist`, true)
		return
	}

	r.output(fmt.Sprintf(`Mode: %s`, params.Mode), true)
	r.output(fmt.Sprintf(`Causally ordered: %t`, params.OrderEnabled), false)
	r.output(fmt.Sprintf(`Virtual synchrony at join: %t`, params.JoinConsistent), false)
	r.output(fmt.Sprintf(`Number of members: %d`, len(mems)), false)
	r.output(fmt.Sprintf(`Member list: %v`, mems), false)
}

/* command-line specific functions */

func (r *runner) input(label string) (input string) {
readInput:
	fmt.Printf("   ? %s: ", label)
	msg, err := r.reader.ReadString('\n')
	if err != nil {
		fmt.Printf("   ! Error: reading %s failed, please try again\n", label)
		goto readInput
	}
	return strings.TrimSpace(msg)
}

func (r *runner) output(text string, firstLine bool) {
	if firstLine {
		fmt.Printf("-> %s\n", text)
		return
	}
	fmt.Printf("   %s\n", text)
}

func (r *runner) outputList(title string, list []string) {
	var out string
	for i, line := range list {
		out += fmt.Sprintf("    %d. %s\n", i+1, line)
	}
	fmt.Printf(fmt.Sprintf("-> %s:\n%s\n", title, out))
}

func (r *runner) error(cmdOut string, err error) {
	fmt.Printf("   ! Error: %s\n", cmdOut)
	if err != nil {
		r.log.Error(err)
	}
}

func (r *runner) cancelCmd(input string) bool {
	if strings.TrimSpace(input) == `b` || strings.TrimSpace(input) == `B` {
		return true
	}
	return false
}

func (r *runner) validBool(input string) (output bool, err error) {
	if strings.ToLower(input) == `y` {
		return true, nil
	} else if strings.ToLower(input) != `n` {
		return false, fmt.Errorf(`invalid input for publisher`)
	}
	return false, nil
}
