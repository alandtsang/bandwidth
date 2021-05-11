package main

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/vishvananda/netlink"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
)

// BandwidthEntry corresponds to a single entry in the bandwidth argument,
// see CONVENTIONS.md
type BandwidthEntry struct {
	IngressRate  uint64 `json:"ingressRate"`  //Bandwidth rate in bps for traffic through container. 0 for no limit. If ingressRate is set, ingressBurst must also be set
	IngressBurst uint64 `json:"ingressBurst"` //Bandwidth burst in bits for traffic through container. 0 for no limit. If ingressBurst is set, ingressRate must also be set

	EgressRate  uint64 `json:"egressRate"`  //Bandwidth rate in bps for traffic through container. 0 for no limit. If egressRate is set, egressBurst must also be set
	EgressBurst uint64 `json:"egressBurst"` //Bandwidth burst in bits for traffic through container. 0 for no limit. If egressBurst is set, egressRate must also be set
}

func (bw *BandwidthEntry) isZero() bool {
	return bw.IngressBurst == 0 && bw.IngressRate == 0 && bw.EgressBurst == 0 && bw.EgressRate == 0
}

// PluginConf is bandwidth plugin configuration structure.
type PluginConf struct {
	types.NetConf

	RuntimeConfig struct {
		Bandwidth *BandwidthEntry `json:"bandwidth,omitempty"`
	} `json:"runtimeConfig,omitempty"`

	*BandwidthEntry
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.PluginSupports("0.3.0", "0.3.1", version.Current()), bv.BuildString("bandwidth"))
}

func cmdAdd(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	bandwidth := getBandwidth(conf)
	if bandwidth == nil || bandwidth.isZero() {
		return types.PrintResult(conf.PrevResult, conf.CNIVersion)
	}

	if conf.PrevResult == nil {
		return fmt.Errorf("must be called as chained plugin")
	}

	result, err := current.NewResultFromResult(conf.PrevResult)
	if err != nil {
		return fmt.Errorf("could not convert result to current version: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	var peerIndex int
	_ = netns.Do(func(_ ns.NetNS) error {
		containerName := result.Interfaces[0].Name
		// container
		containerLink, err := netlink.LinkByName(containerName)
		if err != nil {
			return err
		}
		if bandwidth.EgressRate > 0 {
			err = CreateQdisc(bandwidth.EgressRate, containerLink.Attrs().Name)
			if err != nil {
				return err
			}
		}

		_, peerIndex, err = ip.GetVethPeerIfindex(containerName)
		if err != nil {
			return err
		}
		if peerIndex <= 0 {
			return fmt.Errorf("container interface %s has no veth peer: %v", containerName, err)
		}
		return nil
	})

	// lxc
	lxcLink, err := netlink.LinkByIndex(peerIndex)
	if err != nil {
		return err
	}
	if bandwidth.IngressRate > 0 {
		err = CreateQdisc(bandwidth.IngressRate, lxcLink.Attrs().Name)
		if err != nil {
			return err
		}
	}
	return types.PrintResult(result, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	_, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	bwConf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	if bwConf.PrevResult == nil {
		return fmt.Errorf("must be called as a chained plugin")
	}

	_, err = current.NewResultFromResult(bwConf.PrevResult)
	if err != nil {
		return fmt.Errorf("could not convert result to current version: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	return nil
}

// parseConfig parses the supplied configuration (and prevResult) from stdin.
func parseConfig(stdin []byte) (*PluginConf, error) {
	conf := PluginConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	bandwidth := getBandwidth(&conf)
	if bandwidth != nil {
		err := validateRateAndBurst(bandwidth.IngressRate, bandwidth.IngressBurst)
		if err != nil {
			return nil, err
		}
		err = validateRateAndBurst(bandwidth.EgressRate, bandwidth.EgressBurst)
		if err != nil {
			return nil, err
		}
	}

	if conf.RawPrevResult != nil {
		var err error
		if err = version.ParsePrevResult(&conf.NetConf); err != nil {
			return nil, fmt.Errorf("could not parse prevResult: %v", err)
		}

		_, err = current.NewResultFromResult(conf.PrevResult)
		if err != nil {
			return nil, fmt.Errorf("could not convert result to current version: %v", err)
		}
	}

	return &conf, nil
}

func getBandwidth(conf *PluginConf) *BandwidthEntry {
	if conf.BandwidthEntry == nil && conf.RuntimeConfig.Bandwidth != nil {
		return conf.RuntimeConfig.Bandwidth
	}
	return conf.BandwidthEntry
}

func validateRateAndBurst(rate, burst uint64) error {
	switch {
	case burst < 0 || rate < 0:
		return fmt.Errorf("rate and burst must be a positive integer")
	case burst == 0 && rate != 0:
		return fmt.Errorf("if rate is set, burst must also be set")
	case rate == 0 && burst != 0:
		return fmt.Errorf("if burst is set, rate must also be set")
	case burst/8 >= math.MaxUint32:
		return fmt.Errorf("burst cannot be more than 4GB")
	}

	return nil
}

// CreateQdisc uses tc to set qdisc with bandwidth limit for device.
func CreateQdisc(rateInBits uint64, deviceName string) error {
	device, err := netlink.LinkByName(deviceName)
	if err != nil {
		return fmt.Errorf("get host device: %s", err)
	}
	return createQdisc(device.Attrs().Index, rateInBits)
}

func createQdisc(linkIndex int, rateInBits uint64) (err error) {
	if rateInBits <= 0 {
		return fmt.Errorf("invalid rate: %d", rateInBits)
	}

	if err = createHTBQdisc(linkIndex); err != nil {
		return fmt.Errorf("create htb qdisc failed: %s", err)
	}
	if err = createHTBRootClass(linkIndex, rateInBits); err != nil {
		return fmt.Errorf("create htb root qdisc class failed: %s", err)
	}
	if err = createHTBRootClass2(linkIndex, rateInBits); err != nil {
		return fmt.Errorf("create htb root qdisc class 2 failed: %s", err)
	}
	return nil
}

func createHTBQdisc(linkIndex int) error {
	attrs := netlink.QdiscAttrs{
		LinkIndex: linkIndex,
		Parent:    netlink.HANDLE_ROOT,
		Handle:    netlink.MakeHandle(1, 0),
	}
	qdisc := netlink.NewHtb(attrs)
	if err := netlink.QdiscAdd(qdisc); err != nil {
		return fmt.Errorf("create htb qdisc: %s", err)
	}
	return nil
}

func createHTBRootClass(linkIndex int, rateInBytes uint64) error {
	classattrs := netlink.ClassAttrs{
		LinkIndex: linkIndex,
		Parent:    netlink.MakeHandle(1, 0),
		Handle:    netlink.MakeHandle(1, 1),
	}

	htbclassattrs := netlink.HtbClassAttrs{
		Rate: rateInBytes,
	}
	class := netlink.NewHtbClass(classattrs, htbclassattrs)
	if err := netlink.ClassAdd(class); err != nil {
		return fmt.Errorf("create htb class: %s", err)
	}
	return nil
}

func createHTBRootClass2(linkIndex int, rateInBytes uint64) error {
	classattrs := netlink.ClassAttrs{
		LinkIndex: linkIndex,
		Parent:    netlink.MakeHandle(1, 1),
		Handle:    netlink.MakeHandle(1, 0),
	}

	htbclassattrs := netlink.HtbClassAttrs{
		Rate: rateInBytes,
	}
	class := netlink.NewHtbClass(classattrs, htbclassattrs)
	if err := netlink.ClassAdd(class); err != nil {
		return fmt.Errorf("create htb root class: %s", err)
	}
	return nil
}
