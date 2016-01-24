package testutil

import "sourcegraph.com/sourcegraph/appdash/internal/wire"

type CollectPackets []*wire.CollectPacket

func (cp CollectPackets) Len() int           { return len(cp) }
func (cp CollectPackets) Swap(i, j int)      { cp[i], cp[j] = cp[j], cp[i] }
func (cp CollectPackets) Less(i, j int) bool { return *cp[i].Spanid.Trace < *cp[j].Spanid.Trace }
