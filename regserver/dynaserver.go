package regserver

import (
	"errors"
	"fmt"
	"sync"

	"github.com/golang/glog"
	pb "github.com/relab/smartMerge/proto"
	"golang.org/x/net/context"
)

type DynaServer struct {
	Cur    *pb.Blueprint
	CurC   uint32 // This should be the length of cur, not its Gid.
	RState *pb.State
	Next   map[uint32][]*pb.Blueprint
	mu     sync.RWMutex
}

func (ds *DynaServer) PrintState(op string) {
	fmt.Println("Did operation :", op)
	fmt.Println("New State:")
	fmt.Println("Cur ", ds.Cur)
	fmt.Println("CurC ", ds.CurC)
	fmt.Println("RState ", ds.RState)
	fmt.Println("Next", ds.Next)
}

func NewDynaServer() *DynaServer {
	return &DynaServer{
		RState: &pb.State{make([]byte, 0), int32(0), uint32(0)},
		Next:   make(map[uint32][]*pb.Blueprint, 0),
		mu:     sync.RWMutex{},
	}
}

func NewDynaServerWithCur(cur *pb.Blueprint, curc uint32) *DynaServer {
	return &DynaServer{
		Cur:    cur,
		CurC:   curc,
		RState: &pb.State{make([]byte, 0), int32(0), uint32(0)},
		Next:   make(map[uint32][]*pb.Blueprint, 0),
		mu:     sync.RWMutex{},
	}
}

func (rs *DynaServer) DSetCur(ctx context.Context, nc *pb.NewCur) (*pb.NewCurReply, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	glog.V(5).Infoln("Handling DSetCur")

	if nc.CurC == rs.CurC {
		return &pb.NewCurReply{false}, nil
	}

	if nc.CurC == 0 || nc.Cur.Compare(rs.Cur) == 1 {
		return &pb.NewCurReply{false}, nil
	}

	if rs.Cur != nil && rs.Cur.Compare(nc.Cur) == 0 {
		return &pb.NewCurReply{false}, errors.New("New Current Blueprint was uncomparable to previous.")
	}

	rs.Cur = nc.Cur
	rs.CurC = nc.CurC
	return &pb.NewCurReply{true}, nil
}

func (rs *DynaServer) DWriteN(ctx context.Context, rr *pb.DRead) (*pb.DReadReply, error) {
	if rr.Prop == nil {
		rs.mu.RLock()
		defer rs.mu.RUnlock()
		glog.V(5).Infoln("Handling Empty WriteN")

		if rr.Conf.Cur < rs.CurC {
			return &pb.DReadReply{Cur: rs.Cur}, nil
		}

		return &pb.DReadReply{State: rs.RState, Next: rs.Next[rr.Conf.This]}, nil

	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	glog.V(5).Infoln("Handling WriteN")

	if rr.Conf.Cur < rs.CurC {
		return &pb.DReadReply{Cur: rs.Cur}, nil
	}

	if rr.Prop != nil {
		if len(rs.Next[rr.Conf.This]) > 0 {
			rs.Next[rr.Conf.This] = append(rs.Next[rr.Conf.This], rr.Prop)
		} else {
			rs.Next[rr.Conf.This] = []*pb.Blueprint{rr.Prop}
		}
	}

	return &pb.DReadReply{State: rs.RState, Next: rs.Next[rr.Conf.This]}, nil
}

func (rs *DynaServer) DSetState(ctx context.Context, ns *pb.DNewState) (*pb.NewStateReply, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	glog.V(4).Infoln("Handling SetState")

	if ns.Conf.Cur < rs.CurC {
		// Outdated
		return &pb.NewStateReply{Cur: rs.Cur}, nil
	}

	if rs.RState.Compare(ns.State) == 1 {
		rs.RState = ns.State
	}

	if rs.CurC < ns.Conf.Cur {
		glog.V(4).Infof("New Cur has length %d, previous has length %d\n", ns.Conf.Cur, rs.CurC)
		rs.CurC = ns.Conf.Cur
		rs.Cur = ns.Cur
	}
	return &pb.NewStateReply{Next: rs.Next[ns.Conf.This]}, nil
}

func (rs *DynaServer) DWriteNSet(ctx context.Context, wr *pb.DWriteNs) (*pb.DWriteNsReply, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	glog.V(4).Infoln("Handling WriteNSet")

	if wr.Conf.Cur < rs.CurC {
		glog.V(4).Infoln("CLient has outdated cur.")
		return &pb.DWriteNsReply{Cur: rs.Cur}, nil
	}

	if rs.Next[wr.Conf.This] == nil {
		rs.Next[wr.Conf.This] = wr.Next
	}
outerLoop:
	for _, newBp := range wr.Next {
		for _, bp := range rs.Next[wr.Conf.This] {
			if bp.Equals(newBp) {
				continue outerLoop
			}
		}
		rs.Next[wr.Conf.This] = append(rs.Next[wr.Conf.This], newBp)
	}

	return &pb.DWriteNsReply{Next: rs.Next[wr.Conf.This]}, nil
}

func (rs *DynaServer) GetOneN(ctx context.Context, gt *pb.GetOne) (gtr *pb.GetOneReply, err error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	glog.V(5).Infoln("Handling GetOne")

	if gt.Conf.Cur < rs.CurC {
		return &pb.GetOneReply{Cur: rs.Cur}, nil
	}

	if len(rs.Next[gt.Conf.This]) == 0 {
		rs.Next[gt.Conf.This] = []*pb.Blueprint{gt.Next}
	}

	return &pb.GetOneReply{Next: rs.Next[gt.Conf.This][0]}, nil
}

func (ds *DynaServer) CheckNext(curc uint32, op string) {
	if ds.Next[curc] == nil {
		return
	}
	for _, pb := range ds.Next[curc] {
		if pb == nil {
			fmt.Println("found nil in bp slice, doing ", op)
		}
	}
}