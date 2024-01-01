package box

import (
	"fmt"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/datastructure"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

func (s *Box) startOutboundsAndOutboundProviders() error {
	graph := datastructure.NewGraph[string, any]()
	startedOutboundMap := make(map[string]adapter.Outbound)
	for _, out := range s.outbounds {
		_, isStarter := out.(common.Starter)
		if !isStarter {
			startedOutboundMap[out.Tag()] = out
			continue
		}
		// Add To Graph
		// Check Duplicate
		node := graph.GetNode(graphOutboundTag(out.Tag()))
		if node != nil {
			// Maybe Ghost Outbound
			data := node.Data()
			if data != nil {
				return E.New("outbound/", out.Type(), "[", out.Tag(), "] already exists")
			}
		} else {
			node = datastructure.NewGraphNode[string, any](graphOutboundTag(out.Tag()), out)
			graph.AddNode(node)
		}
		// Dependencies
		dependencies := out.Dependencies()
		for _, dependency := range dependencies {
			// Check is unstart Outbound
			_, loaded := startedOutboundMap[dependency]
			if loaded {
				continue
			}
			// Search Graph
			dpNode := graph.GetNode(graphOutboundTag(dependency))
			if dpNode == nil {
				// Maybe Ghost Outbound
				dpNode = datastructure.NewGraphNode[string, any](graphOutboundTag(dependency), nil)
				graph.AddNode(dpNode)
			}
			dpNode.AddNext(node)
			node.AddPrev(dpNode)
		}
	}
	for _, provider := range s.outboundProviders {
		// Add To Graph
		// Check Duplicate
		node := graph.GetNode(graphOutboundProviderTag(provider.Tag()))
		if node != nil {
			return E.New("outbound-provider[", provider.Tag(), "] already exists")
		} else {
			node = datastructure.NewGraphNode[string, any](graphOutboundProviderTag(provider.Tag()), provider)
			graph.AddNode(node)
		}
		dependentOutbound := provider.DependentOutbound()
		if dependentOutbound != "" {
			// Check is unstart Outbound
			_, loaded := startedOutboundMap[dependentOutbound]
			if !loaded {
				// Search Graph
				dpNode := graph.GetNode(graphOutboundTag(dependentOutbound))
				if dpNode == nil {
					// Maybe Ghost Outbound
					dpNode = datastructure.NewGraphNode[string, any](graphOutboundTag(dependentOutbound), nil)
					graph.AddNode(dpNode)
				}
				dpNode.AddNext(node)
				node.AddPrev(dpNode)
			}
		}
	}
	queue := datastructure.NewQueue[*datastructure.GraphNode[string, any]]()
	monitor := taskmonitor.New(s.logger, C.DefaultStartTimeout)
	for {
		for queue.Len() > 0 {
			node := queue.Pop()
			data := node.Data()
			switch out := data.(type) {
			case adapter.Outbound:
				monitor.Start("initialize outbound/", out.Type(), "[", out, "]")
				err := out.(common.Starter).Start()
				monitor.Finish()
				if err != nil {
					return E.Cause(err, "initialize outbound/", out.Type(), "[", out.Tag(), "]")
				}
				startedOutboundMap[out.Tag()] = out
			case adapter.OutboundProvider:
				monitor.Start("pre-start outbound-provider[", out.Tag(), "]")
				err := out.PreStart()
				monitor.Finish()
				if err != nil {
					return E.Cause(err, "pre-start outbound-provider[", out.Tag(), "]")
				}
				outbounds := out.Outbounds()
				for _, outbound := range outbounds {
					_, isStarter := outbound.(common.Starter)
					if !isStarter {
						// Check Duplicate
						_, loaded := startedOutboundMap[outbound.Tag()]
						if loaded {
							return E.New("outbound/", outbound.Type(), "[", outbound.Tag(), "] already exists")
						}
						startedOutboundMap[outbound.Tag()] = outbound
						continue
					}
					outboundNode := graph.GetNode(graphOutboundTag(outbound.Tag()))
					if outboundNode == nil {
						outboundNode = datastructure.NewGraphNode[string, any](graphOutboundTag(outbound.Tag()), outbound)
						graph.AddNode(outboundNode)
					} else {
						// Check Duplicate
						if outboundNode.Data() != nil {
							return E.New("outbound/", outbound.Type(), "[", outbound.Tag(), "] already exists")
						}
						// Maybe Ghost Outbound: SetData
						outboundNode.SetData(outbound)
					}
					dependencies := outbound.Dependencies()
					if len(dependencies) == 0 {
						queue.Push(outboundNode)
						continue
					}
					for _, dependency := range dependencies {
						// Check is unstart Outbound
						_, loaded := startedOutboundMap[dependency]
						if loaded {
							continue
						}
						// Search Graph
						dpNode := graph.GetNode(graphOutboundTag(dependency))
						if dpNode == nil {
							// Maybe Ghost Outbound
							dpNode = datastructure.NewGraphNode[string, any](graphOutboundTag(dependency), nil)
							graph.AddNode(dpNode)
						}
						dpNode.AddNext(outboundNode)
						outboundNode.AddPrev(dpNode)
					}
				}
				graph.RemoveNode(node.ID())
			}
			for _, next := range node.Next() {
				next.RemovePrev(node)
			}
		}
		for _, node := range graph.NodeMap() {
			if len(node.Prev()) == 0 && node.Data() != nil {
				if strings.HasPrefix(node.ID(), "outbound-") {
					tag := strings.TrimPrefix(node.ID(), "outbound-")
					_, loaded := startedOutboundMap[tag]
					if loaded {
						continue
					}
				}
				queue.Push(node)
			}
		}
		if queue.Len() == 0 {
			break
		}
	}
	circles := graph.FindCircle()
	if len(circles) > 0 {
		// Print First
		firstCircle := circles[0]
		var s string
		for _, id := range firstCircle {
			var ss string
			switch {
			case strings.HasPrefix(id, "outbound-"):
				ss = fmt.Sprintf("outbound[%s]", strings.TrimPrefix(id, "outbound-"))
			case strings.HasPrefix(id, "outbound-provider-"):
				ss = fmt.Sprintf("outbound-provider[%s]", strings.TrimPrefix(id, "outbound-provider-"))
			}
			s += ss + " -> "
		}
		switch {
		case strings.HasPrefix(firstCircle[0], "outbound-"):
			s = fmt.Sprintf("outbound[%s] -> ", strings.TrimPrefix(firstCircle[0], "outbound-")) + s
		case strings.HasPrefix(firstCircle[0], "outbound-provider-"):
			s = fmt.Sprintf("outbound-provider[%s] -> ", strings.TrimPrefix(firstCircle[0], "outbound-provider-")) + s
		}
		return E.New("circular dependency: ", s)
	}
	// Maybe Ghost Outbound
	for _, node := range graph.NodeMap() {
		if node.Data() != nil {
			continue
		}
		id := node.ID()
		outTag := strings.TrimPrefix(id, "outbound-")
		return E.New("outbound [", outTag, "] not found")
	}
	return nil
}

func graphOutboundTag(s string) string {
	return "outbound-" + s
}

func graphOutboundProviderTag(s string) string {
	return "outbound-provider-" + s
}
