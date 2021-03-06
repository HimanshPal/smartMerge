package regserver

import (
	"encoding/binary"
	"fmt"
	"testing"

	pb "github.com/relab/smartMerge/proto"
	"golang.org/x/net/context"
	//"google.golang.org/grpc"
)

var ctx = context.Background()
var bytes = make([]byte, 64)

var one = uint32(1)
var two = uint32(2)
var tre = uint32(3)

var n11 = &pb.Node{one, one}
var n12 = &pb.Node{one, two}
var n22 = &pb.Node{two, two}
var n32 = &pb.Node{tre, two}
var n33 = &pb.Node{tre, tre}

var b1 = &pb.Blueprint{[]*pb.Node{n11}, one, one}
var b2 = &pb.Blueprint{[]*pb.Node{n22}, two, one}
var b12 = &pb.Blueprint{[]*pb.Node{n11, n22}, two, one}
var b22 = &pb.Blueprint{[]*pb.Node{n11, n22}, two, two}
var b23 = &pb.Blueprint{[]*pb.Node{n11, n22}, tre, two}

var b12x = &pb.Blueprint{[]*pb.Node{n12, n22}, two, one}
var b123 = &pb.Blueprint{[]*pb.Node{n12, n22, n32}, two, one}
var bx = &pb.Blueprint{[]*pb.Node{n11, n33}, tre, two}
var by = &pb.Blueprint{[]*pb.Node{n12, n32}, two, one}
var b0 *pb.Blueprint

func Put(x int, bytes []byte) []byte {
	binary.PutUvarint(bytes, uint64(x))
	return bytes
}

func Get(bytes []byte) int {
	x, _ := binary.Uvarint(bytes)
	return int(x)
}

func TestSetState(t *testing.T) {
	rs := NewRegServer(false)
	rs.Next = []*pb.Blueprint{b12, b12x}
	//Perfectly normal SetState
	stest, err := rs.SetState(ctx, &pb.NewState{
		Cur:     b2,
		CurC:    uint32(b2.Len()),
		State:   &pb.State{Value: nil, Timestamp: 2, Writer: 0},
		LAState: b1,
	})
	if err != nil || rs.Cur != b2 || rs.CurC != uint32(b2.Len()) || rs.RState.Compare(&pb.State{nil, 2, 0}) != 0 || !rs.LAState.Equals(b1) {
		t.Error("first write did not work")
	}
	if len(stest.Next) != 2 {
		t.Error("did not return correct next")
	}

	// Set state in Cur.
	stest, _ = rs.SetState(ctx, &pb.NewState{
		Cur:     b2,
		CurC:    uint32(b2.Len()),
		State:   &pb.State{Value: nil, Timestamp: 2, Writer: 1},
		LAState: b2,
	})
	if rs.Cur != b2 || rs.CurC != uint32(b2.Len()) || rs.RState.Compare(&pb.State{nil, 2, 1}) != 0 || !rs.LAState.Equals(b12) {
		t.Error("did not set state correctly")
	}
	if len(stest.Next) != 2 {
		t.Error("did not return correct next")
	}
	if stest.Cur != nil {
		t.Error("did return wrong cur")
	}

	// Clean next on set state
	stest, _ = rs.SetState(ctx, &pb.NewState{
		Cur:     b12,
		CurC:    uint32(b12.Len()),
		LAState: b12x,
	})
	if rs.Cur != b12 || rs.CurC != uint32(b12.Len()) || rs.RState.Compare(&pb.State{nil, 2, 1}) != 0 || !rs.LAState.Equals(b12x) {
		t.Error("did not set state correctly")
	}
	if len(rs.Next) != 1 {
		t.Error("did not clean up Next")
	}
	if len(stest.Next) != 1 {
		t.Error("did not return correct next")
	}
	if stest.Cur != nil {
		t.Error("did return wrong cur")
	}

	// Set state in old cur
	stest, _ = rs.SetState(ctx, &pb.NewState{
		Cur:     b2,
		CurC:    uint32(b2.Len()),
		State:   &pb.State{nil, 3, 0},
		LAState: b123,
	})
	if rs.Cur != b12 || rs.CurC != uint32(b12.Len()) || rs.RState.Compare(&pb.State{nil, 3, 0}) != 0 || !rs.LAState.Equals(b123) {
		t.Error("did not set state correctly")
	}
	if len(rs.Next) != 1 {
		t.Error("did not clean up Next")
	}
	if stest.Cur != b12 {
		t.Error("did return wrong cur")
	}
}

func TestLAProp(t *testing.T) {
	rs := NewRegServer(false)
	var bytes = make([]byte, 64)
	bytes = Put(5, bytes)
	rs.Next = []*pb.Blueprint{b12}

	// Test it returns no error and writes
	stest, err := rs.LAProp(ctx, &pb.LAProposal{Prop: b12, Conf: &pb.Conf{}})
	if err != nil {
		t.Error("Did return error")
	}
	if rs.LAState != b12 {
		t.Error("did not write")
	}
	if stest.LAState != nil {
		t.Error("did return LAState")
	}
	if len(stest.Next) != 1 {
		t.Error("wrong next")
	}

	rs.Cur = b2
	rs.CurC = uint32(b2.Len())

	//Can abort
	stest, _ = rs.LAProp(ctx, &pb.LAProposal{Prop: b12x, Conf: &pb.Conf{one, one}})
	if rs.LAState != b12 {
		t.Error("did write on abort")
	}
	if !stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("laprop did return correct abort")
	}

	//Does not abort, but return cur, does not write old value.
	stest, _ = rs.LAProp(ctx, &pb.LAProposal{Prop: b2, Conf: &pb.Conf{Cur: one, This: uint32(b2.Len())}})
	if stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("laprop did not return correct cur.")
	}
	if !stest.LAState.Equals(b12) {
		//fmt.Println(stest.LAState)
		t.Error("did not return LAState")
	}
	if !rs.LAState.Equals(b12) {
		t.Error("wrong state")
	}

	// If noabort is true, does not abort, but sends cur, state and next.
	rs.Next = []*pb.Blueprint{b12, b12x}
	rs.noabort = true
	stest, _ = rs.LAProp(ctx, &pb.LAProposal{Prop: by, Conf: &pb.Conf{one, one}})
	if stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("laprop did not return correct cur.")
	}
	if !rs.LAState.Equals(b123) {
		t.Error("laprop did not write correctly")
	}
	if !stest.LAState.Equals(b123) {
		t.Error("did not return LAState")
	}

	// Only send next that is large.
	stest, _ = rs.LAProp(ctx, &pb.LAProposal{Prop: bx, Conf: &pb.Conf{uint32(b12.Len()), uint32(b12.Len())}})
	if stest.Cur != nil {
		t.Error("laprop did not return correct cur.")
	}
	if !rs.LAState.Equals(bx.Merge(b123)) {
		t.Error("laprop did not result in correct state.")
	}
	if !stest.LAState.Equals(bx.Merge(b123)) {
		t.Error("laprop did not return correct state.")
	}

}

func TestAWriteN(t *testing.T) {
	rs := NewRegServer(false)
	var bytes = make([]byte, 64)
	bytes = Put(5, bytes)
	s := &pb.State{Value: bytes, Timestamp: 2, Writer: 0}

	// Test it returns no error and writes
	stest, err := rs.AWriteN(ctx, &pb.WriteN{Next: b12})
	if err != nil {
		t.Error("Did return error")
	}
	if len(rs.Next) != 1 {
		t.Error("did not write")
	}

	rs.Cur = b2
	rs.CurC = uint32(b2.Len())
	rs.LAState = b12x
	rs.RState = s

	//Can abort
	stest, _ = rs.AWriteN(ctx, &pb.WriteN{Next: b12x, CurC: one})
	if len(rs.Next) != 1 {
		t.Error("did write next on abort")
	}
	if !stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("writeN did return correct abort")
	}

	//Does not abort, does not write duplicate next.
	stest, _ = rs.AWriteN(ctx, &pb.WriteN{Next: b12, CurC: uint32(b2.Len())})
	if stest.Cur != nil {
		t.Error("writeN did not return correct cur.")
	}
	if stest.State != s {
		t.Error("writeN did not return state")
	}
	if len(rs.Next) != 1 {
		t.Error("did write duplicate next")
	}
	if stest.LAState != b12x {
		t.Error("did not return LAState")
	}
	if len(stest.Next) != 1 {
		t.Error("writeN did not return correct next")
	}

	// If noabort is true, does not abort, but sends cur, state and next.
	rs.noabort = true
	stest, _ = rs.AWriteN(ctx, &pb.WriteN{Next: b12x, CurC: one})
	if stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("writeN did not return correct cur.")
	}
	if stest.State != s {
		t.Error("writeN returned wrong state")
	}
	if len(stest.Next) != 2 {
		t.Error("writeN did not return correct Next")
	}
	if len(rs.Next) != 2 {
		t.Error("writeN did not write correctly")
	}
	if stest.LAState != b12x {
		t.Error("did not return LAState")
	}

	// Only send next that is large.
	stest, _ = rs.AWriteN(ctx, &pb.WriteN{CurC: uint32(b12.Len())})
	if stest.Cur != nil {
		t.Error("writeN did not return correct cur.")
	}
	if len(stest.Next) != 1 {
		t.Error("writeN did not return correct Next")
	}
}

func TestWriteAWriteS(t *testing.T) {
	rs := NewRegServer(false)
	var bytes = make([]byte, 64)
	bytes = Put(5, bytes)
	s := &pb.State{Value: bytes, Timestamp: 2, Writer: 0}

	// Test it returns no error and writes
	stest, err := rs.AWriteS(ctx, &pb.WriteS{State: s})
	if err != nil {
		t.Error("Did return error")
	}
	if rs.RState != s {
		t.Error("did not write")
	}

	s0 := &pb.State{Value: nil, Timestamp: 1, Writer: 0}
	rs.Cur = b2
	rs.CurC = uint32(b2.Len())

	//Can abort
	stest, _ = rs.AWriteS(ctx, &pb.WriteS{State: s0, Conf: &pb.Conf{one, one}})
	if rs.RState == s0 {
		t.Error("did write value with smaller timestamp")
	}
	if !stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("writeS did return correct abort")
	}

	//Does not abort, but sends cur, and new state.
	s2 := &pb.State{Value: nil, Timestamp: 2, Writer: 1}
	stest, _ = rs.AWriteS(ctx, &pb.WriteS{State: s2, Conf: &pb.Conf{Cur: one, This: uint32(b2.Len())}})
	if stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("writeS did not return correct cur.")
	}
	if rs.RState != s2 {
		t.Error("writeS did not write")
	}

	// If noabort is true, does not abort, but sends cur, state and next.
	s3 := &pb.State{Value: nil, Timestamp: 3, Writer: 0}
	rs.noabort = true
	rs.Next = []*pb.Blueprint{b12, b12x}
	stest, _ = rs.AWriteS(ctx, &pb.WriteS{State: s3, Conf: &pb.Conf{Cur: one, This: one}})
	if stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("writeS did not return correct cur.")
	}
	if rs.RState != s3 {
		t.Error("writeS returned wrong state")
	}
	if len(stest.Next) != 2 {
		t.Error("writeS did not return correct Next")
	}

	// Only send next that is large.
	stest, _ = rs.AWriteS(ctx, &pb.WriteS{Conf: &pb.Conf{uint32(b12.Len()), uint32(b12.Len())}})
	if stest.Cur != nil {
		t.Error("writeS did not return correct cur.")
	}
	if len(stest.Next) != 1 {
		t.Error("writeS did not return correct Next")
	}
}

func TestWriteAReadS(t *testing.T) {
	rs := NewRegServer(false)
	var bytes = make([]byte, 64)
	bytes = Put(5, bytes)
	s := &pb.State{bytes, 2, 0}

	// Test it returns no error
	stest, err := rs.AReadS(ctx, &pb.Conf{})
	if err != nil {
		t.Error("Did return error")
	}
	fmt.Printf("Direct ReadS returned: %v\n", stest)
	fmt.Printf("Should return: %v\n", &InitState)

	rs.RState = s
	rs.Cur = b2
	rs.CurC = uint32(b2.Len())

	//Can abort
	stest, _ = rs.AReadS(ctx, &pb.Conf{one, one})
	if !stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("read S did return correct abort")
	}

	//Does not abort, but sends cur, and new state.
	stest, _ = rs.AReadS(ctx, &pb.Conf{Cur: one, This: uint32(b2.Len())})
	if stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("read S did not return correct cur.")
	}
	if stest.State.Compare(s) != 0 {
		t.Error("readS returned wrong state")
	}

	// If noabort is true, does not abort, but sends cur, state and next.
	rs.noabort = true
	rs.Next = []*pb.Blueprint{b12, b12x}
	stest, _ = rs.AReadS(ctx, &pb.Conf{one, one})
	if stest.Cur.Abort || stest.Cur.Cur != b2 {
		t.Error("read S did not return correct cur.")
	}
	if stest.State.Compare(s) != 0 {
		t.Error("readS returned wrong state")
	}
	if len(stest.Next) != 2 {
		t.Error("readS did not return correct Next")
	}

	// Only send next that is large.
	stest, _ = rs.AReadS(ctx, &pb.Conf{uint32(b12.Len()), uint32(b12.Len())})
	if stest.Cur != nil {
		t.Error("read S did not return correct cur.")
	}
	if stest.State.Compare(s) != 0 {
		t.Error("readS returned wrong state")
	}
	if len(stest.Next) != 1 {
		t.Error("readS did not return correct Next")
	}

}
