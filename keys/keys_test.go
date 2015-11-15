// Copyright 2014 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.
//
// Author: Tobias Schottdorf (tobias.schottdorf@gmail.com)

package keys

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/roachpb"
	"github.com/cockroachdb/cockroach/testutils"
	"github.com/cockroachdb/cockroach/util/encoding"
	"github.com/cockroachdb/cockroach/util/leaktest"
	"github.com/cockroachdb/cockroach/util/uuid"
)

// TestLocalKeySorting is a sanity check to make sure that
// the non-replicated part of a store sorts before the meta.
func TestKeySorting(t *testing.T) {
	defer leaktest.AfterTest(t)
	// Reminder: Increasing the last byte by one < adding a null byte.
	if !(roachpb.RKey("").Less(roachpb.RKey("\x00")) && roachpb.RKey("\x00").Less(roachpb.RKey("\x01")) &&
		roachpb.RKey("\x01").Less(roachpb.RKey("\x01\x00"))) {
		t.Fatalf("something is seriously wrong with this machine")
	}
	if bytes.Compare(localPrefix, MetaPrefix) >= 0 {
		t.Fatalf("local key spilling into replicated ranges")
	}
	if !bytes.Equal(roachpb.Key(""), roachpb.Key(nil)) || !bytes.Equal(roachpb.Key(""), roachpb.Key(nil)) {
		t.Fatalf("equality between keys failed")
	}
}

func TestMakeKey(t *testing.T) {
	defer leaktest.AfterTest(t)
	if !bytes.Equal(MakeKey(roachpb.Key("A"), roachpb.Key("B")), roachpb.Key("AB")) ||
		!bytes.Equal(MakeKey(roachpb.Key("A")), roachpb.Key("A")) ||
		!bytes.Equal(MakeKey(roachpb.Key("A"), roachpb.Key("B"), roachpb.Key("C")), roachpb.Key("ABC")) {
		t.Fatalf("MakeKey is broken")
	}
}

func TestKeyAddress(t *testing.T) {
	defer leaktest.AfterTest(t)
	testCases := []struct {
		key        roachpb.Key
		expAddress roachpb.RKey
	}{
		{roachpb.Key{}, roachpb.RKeyMin},
		{roachpb.Key("123"), roachpb.RKey("123")},
		{RangeDescriptorKey(roachpb.RKey("foo")), roachpb.RKey("foo")},
		{TransactionKey(roachpb.Key("baz"), uuid.NewUUID4()), roachpb.RKey("baz")},
		{TransactionKey(roachpb.KeyMax, roachpb.RKey(uuid.NewUUID4())), roachpb.RKeyMax},
		{nil, nil},
	}
	for i, test := range testCases {
		result := Addr(test.key)
		if !result.Equal(test.expAddress) {
			t.Errorf("%d: expected address for key %q doesn't match %q", i, test.key, test.expAddress)
		}
	}
}

func TestRangeMetaKey(t *testing.T) {
	defer leaktest.AfterTest(t)
	testCases := []struct {
		key, expKey roachpb.RKey
	}{
		{
			key:    roachpb.RKey{},
			expKey: roachpb.RKeyMin,
		},
		{
			key:    roachpb.RKey("\x00\x00meta2\x00zonefoo"),
			expKey: roachpb.RKey("\x00\x00meta1\x00zonefoo"),
		},
		{
			key:    roachpb.RKey("\x00\x00meta1\x00zonefoo"),
			expKey: roachpb.RKeyMin,
		},
		{
			key:    roachpb.RKey("foo"),
			expKey: roachpb.RKey("\x00\x00meta2foo"),
		},
		{
			key:    roachpb.RKey("foo"),
			expKey: roachpb.RKey("\x00\x00meta2foo"),
		},
		{
			key:    roachpb.RKey("\x00\x00meta2foo"),
			expKey: roachpb.RKey("\x00\x00meta1foo"),
		},
		{
			key:    roachpb.RKey("\x00\x00meta1foo"),
			expKey: roachpb.RKeyMin,
		},
	}
	for i, test := range testCases {
		result := RangeMetaKey(test.key)
		if !bytes.Equal(result, test.expKey) {
			t.Errorf("%d: expected range meta for key %q doesn't match %q", i, test.key, test.expKey)
		}
	}
}

// TestMetaPrefixLen asserts that both levels of meta keys have the same prefix length,
// as MetaScanBounds, MetaReverseScanBounds and validateRangeMetaKey depend on this fact.
func TestMetaPrefixLen(t *testing.T) {
	if len(Meta1Prefix) != len(Meta2Prefix) {
		t.Fatalf("Meta1Prefix %q and Meta2Prefix %q are not of equal length!", Meta1Prefix, Meta2Prefix)
	}
}

func TestMetaScanBounds(t *testing.T) {
	defer leaktest.AfterTest(t)

	testCases := []struct {
		key, expStart, expEnd []byte
		expError              string
	}{
		{
			key:      roachpb.RKey{},
			expStart: Meta1Prefix,
			expEnd:   Meta1Prefix.PrefixEnd(),
			expError: "",
		},
		{
			key:      roachpb.MakeKey(Meta2Prefix, roachpb.Key("foo")),
			expStart: roachpb.MakeKey(Meta2Prefix, roachpb.Key("foo\x00")),
			expEnd:   Meta2Prefix.PrefixEnd(),
			expError: "",
		},
		{
			key:      roachpb.MakeKey(Meta1Prefix, roachpb.Key("foo")),
			expStart: roachpb.MakeKey(Meta1Prefix, roachpb.Key("foo\x00")),
			expEnd:   Meta1Prefix.PrefixEnd(),
			expError: "",
		},
		{
			key:      roachpb.MakeKey(Meta1Prefix, roachpb.RKeyMax),
			expStart: roachpb.MakeKey(Meta1Prefix, roachpb.RKeyMax),
			expEnd:   Meta1Prefix.PrefixEnd(),
			expError: "",
		},
		{
			key:      Meta2KeyMax,
			expStart: nil,
			expEnd:   nil,
			expError: "Meta2KeyMax can't be used as the key of scan",
		},
		{
			key:      Meta2KeyMax.Next(),
			expStart: nil,
			expEnd:   nil,
			expError: "body of meta key range lookup is",
		},
		{
			key:      Meta1KeyMax.Next(),
			expStart: nil,
			expEnd:   nil,
			expError: "body of meta key range lookup is",
		},
	}
	for i, test := range testCases {
		resStart, resEnd, err := MetaScanBounds(test.key)

		if err != nil && !testutils.IsError(err, test.expError) {
			t.Errorf("expected error: %s ; got %s", test.expError, err)
		} else if err == nil && test.expError != "" {
			t.Errorf("expected error: %s", test.expError)
		}

		if !resStart.Equal(test.expStart) || !resEnd.Equal(test.expEnd) {
			t.Errorf("%d: range bounds %q-%q don't match expected bounds %q-%q for key %q", i, resStart, resEnd, test.expStart, test.expEnd, test.key)
		}
	}
}

func TestMetaReverseScanBounds(t *testing.T) {
	defer leaktest.AfterTest(t)

	testCases := []struct {
		key              []byte
		expStart, expEnd []byte
		expError         string
	}{
		{
			key:      roachpb.RKey{},
			expStart: nil,
			expEnd:   nil,
			expError: "KeyMin and Meta1Prefix can't be used as the key of reverse scan",
		},
		{
			key:      Meta1Prefix,
			expStart: nil,
			expEnd:   nil,
			expError: "KeyMin and Meta1Prefix can't be used as the key of reverse scan",
		},
		{
			key:      Meta2KeyMax.Next(),
			expStart: nil,
			expEnd:   nil,
			expError: "body of meta key range lookup is",
		},
		{
			key:      Meta1KeyMax.Next(),
			expStart: nil,
			expEnd:   nil,
			expError: "body of meta key range lookup is",
		},
		{
			key:      roachpb.MakeKey(Meta2Prefix, roachpb.Key("foo")),
			expStart: Meta2Prefix,
			expEnd:   roachpb.MakeKey(Meta2Prefix, roachpb.Key("foo\x00")),
			expError: "",
		},
		{
			key:      roachpb.MakeKey(Meta1Prefix, roachpb.Key("foo")),
			expStart: Meta1Prefix,
			expEnd:   roachpb.MakeKey(Meta1Prefix, roachpb.Key("foo\x00")),
			expError: "",
		},
		{
			key:      Addr(Meta2Prefix),
			expStart: Meta1Prefix,
			expEnd:   Meta2Prefix.Next(),
			expError: "",
		},
		{
			key:      Meta2KeyMax,
			expStart: Meta2Prefix,
			expEnd:   Meta2KeyMax.Next(),
			expError: "",
		},
	}
	for i, test := range testCases {
		resStart, resEnd, err := MetaReverseScanBounds(roachpb.RKey(test.key))

		if err != nil && !testutils.IsError(err, test.expError) {
			t.Errorf("expected error: %s ; got %s", test.expError, err)
		} else if err == nil && test.expError != "" {
			t.Errorf("expected error: %s", test.expError)
		}

		if !resStart.Equal(test.expStart) || !resEnd.Equal(test.expEnd) {
			t.Errorf("%d: range bounds %q-%q don't match expected bounds %q-%q for key %q", i, resStart, resEnd, test.expStart, test.expEnd, test.key)
		}
	}
}

func TestValidateRangeMetaKey(t *testing.T) {
	defer leaktest.AfterTest(t)
	testCases := []struct {
		key    []byte
		expErr bool
	}{
		{roachpb.RKeyMin, false},
		{roachpb.RKey("\x00"), true},
		{Meta1Prefix[:len(Meta1Prefix)-1], true},
		{Meta1Prefix, false},
		{roachpb.MakeKey(Meta1Prefix, roachpb.RKeyMax), false},
		{roachpb.MakeKey(Meta2Prefix, roachpb.RKeyMax), false},
		{roachpb.MakeKey(Meta2Prefix, roachpb.RKeyMax.Next()), true},
	}
	for i, test := range testCases {
		err := validateRangeMetaKey(test.key)
		if err != nil != test.expErr {
			t.Errorf("%d: expected error? %t: %s", i, test.expErr, err)
		}
	}
}

func TestBatchRange(t *testing.T) {
	defer leaktest.AfterTest(t)
	testCases := []struct {
		req [][2]string
		exp [2]string
	}{
		{
			// Boring single request.
			req: [][2]string{{"a", "b"}},
			exp: [2]string{"a", "b"},
		},
		{
			// Request with invalid range. It's important that this still
			// results in a valid range.
			req: [][2]string{{"b", "a"}},
			exp: [2]string{"b", "b\x00"},
		},
		{
			// Two overlapping ranges.
			req: [][2]string{{"a", "c"}, {"b", "d"}},
			exp: [2]string{"a", "d"},
		},
		{
			// Two disjoint ranges.
			req: [][2]string{{"a", "b"}, {"c", "d"}},
			exp: [2]string{"a", "d"},
		},
		{
			// Range and disjoint point request.
			req: [][2]string{{"a", "b"}, {"c", ""}},
			exp: [2]string{"a", "c\x00"},
		},
		{
			// Three disjoint point requests.
			req: [][2]string{{"a", ""}, {"b", ""}, {"c", ""}},
			exp: [2]string{"a", "c\x00"},
		},
		{
			// Disjoint range request and point request.
			req: [][2]string{{"a", "b"}, {"b", ""}},
			exp: [2]string{"a", "b\x00"},
		},
		{
			// Range-local point request.
			req: [][2]string{{string(RangeDescriptorKey(roachpb.RKeyMax)), ""}},
			exp: [2]string{"\xff\xff", "\xff\xff\x00"},
		},
		{
			// Range-local to global such that the key ordering flips.
			// Important that we get a valid range back.
			req: [][2]string{{string(RangeDescriptorKey(roachpb.RKeyMax)), "x"}},
			exp: [2]string{"\xff\xff", "\xff\xff\x00"},
		},
		{
			// Range-local to global without order messed up.
			req: [][2]string{{string(RangeDescriptorKey(roachpb.RKey("a"))), "x"}},
			exp: [2]string{"a", "x"},
		},
	}

	for i, c := range testCases {
		var ba roachpb.BatchRequest
		for _, pair := range c.req {
			ba.Add(&roachpb.ScanRequest{Span: roachpb.Span{Key: roachpb.Key(pair[0]), EndKey: roachpb.Key(pair[1])}})
		}
		rs := Range(ba)
		if actPair := [2]string{string(rs.Key), string(rs.EndKey)}; !reflect.DeepEqual(actPair, c.exp) {
			t.Fatalf("%d: expected [%q,%q), got [%q,%q)", i, c.exp[0], c.exp[1], actPair[0], actPair[1])
		}
	}
}

func TestPrettyPrint(t *testing.T) {
	defer leaktest.AfterTest(t)

	tm, _ := time.Parse(time.UnixDate, "Sat Mar  7 11:06:39 UTC 2015")

	testCases := []struct {
		key roachpb.Key
		exp string
	}{
		// local
		{StoreIdentKey(), "/Local/Store/storeIdent"},
		{ResponseCacheKey(roachpb.RangeID(1000001), []byte("test0")), `/Local/RangeID/1000001/ResponseCache/"test0"`},
		{RaftLeaderLeaseKey(roachpb.RangeID(1000001)), "/Local/RangeID/1000001/RaftLeaderLease"},
		{RaftTombstoneKey(roachpb.RangeID(1000001)), "/Local/RangeID/1000001/RaftTombstone"},
		{RaftHardStateKey(roachpb.RangeID(1000001)), "/Local/RangeID/1000001/RaftHardState"},
		{RaftAppliedIndexKey(roachpb.RangeID(1000001)), "/Local/RangeID/1000001/RaftAppliedIndex"},
		{RaftLogKey(roachpb.RangeID(1000001), uint64(200001)), "/Local/RangeID/1000001/RaftLog/logIndex:200001"},
		{RaftTruncatedStateKey(roachpb.RangeID(1000001)), "/Local/RangeID/1000001/RaftTruncatedState"},
		{RaftLastIndexKey(roachpb.RangeID(1000001)), "/Local/RangeID/1000001/RaftLastIndex"},
		{RangeGCMetadataKey(roachpb.RangeID(1000001)), "/Local/RangeID/1000001/RangeGCMetadata"},
		{RangeLastVerificationTimestampKey(roachpb.RangeID(1000001)), "/Local/RangeID/1000001/RangeLastVerificationTimestamp"},
		{RangeStatsKey(roachpb.RangeID(1000001)), "/Local/RangeID/1000001/RangeStats"},

		{MakeRangeKeyPrefix(roachpb.RKey("ok")), `/Local/Range/"ok"`},
		{RangeDescriptorKey(roachpb.RKey("111")), `/Local/Range/RangeDescriptor/"111"`},
		{RangeTreeNodeKey(roachpb.RKey("111")), `/Local/Range/RangeTreeNode/"111"`},
		{TransactionKey(roachpb.Key("111"), []byte("22222")), `/Local/Range/Transaction/addrKey:/"111"/id:"22222"`},

		{LocalMax, "/Local/Max"},

		{roachpb.MakeKey(localPrefix, []byte("2222")), `/Local/"2222"`},

		// system
		{roachpb.MakeKey(Meta2Prefix, roachpb.Key("foo")), `/System/Meta2/"foo"`},
		{roachpb.MakeKey(Meta1Prefix, roachpb.Key("foo")), `/System/Meta1/"foo"`},

		{StoreStatusKey(2222), "/System/StatusStore/2222"},
		{NodeStatusKey(1111), "/System/StatusNode/1111"},

		{Meta1KeyMax, `/System/Meta1/Max`},
		{Meta2KeyMax, `/System/Meta2/Max`},
		{SystemMax, "/System/Max"},

		{roachpb.MakeKey(SystemPrefix, roachpb.Key("111")), `/System/"111"`},

		// table
		{UserTableDataMin, "/Table/1000"},
		{MakeKey(TableDataPrefix, []byte("a")), `/Table/"a"`},
		{MakeKey(TableDataPrefix, []byte("\xff")), `/Table/"\xff"`},
		{MakeTablePrefix(111), "/Table/111"},
		{MakeKey(MakeTablePrefix(42), roachpb.RKey("foo")), `/Table/42/"foo"`},
		{MakeKey(MakeTablePrefix(42), roachpb.RKey(encoding.EncodeFloat(nil, float64(233.221112)))), "/Table/42/233.221112"},
		{MakeKey(MakeTablePrefix(42), roachpb.RKey(encoding.EncodeVarint(nil, 1222)), roachpb.RKey(encoding.EncodeString(nil, "handsome man"))), `/Table/42/1222/"handsome man"`},
		{MakeKey(MakeTablePrefix(42), roachpb.RKey(encoding.EncodeBytes(nil, []byte{1, 2, 8, 255}))), `/Table/42/"\x01\x02\b\xff"`},
		{MakeKey(MakeTablePrefix(42), roachpb.RKey(encoding.EncodeBytes(nil, []byte{1, 2, 8, 255})), roachpb.RKey("foo")), `/Table/42/"\x01\x02\b\xff"/"foo"`},
		{MakeKey(MakeTablePrefix(42), roachpb.RKey(encoding.EncodeNull(nil))), "/Table/42/NULL"},
		{MakeKey(MakeTablePrefix(42), roachpb.RKey(encoding.EncodeNotNull(nil))), "/Table/42/#"},

		{MakeKey(MakeTablePrefix(42), roachpb.RKey(encoding.EncodeTime(nil, tm))), "/Table/42/Sat Mar  7 11:06:39 UTC 2015"},

		// others
		{MakeKey(MakeTablePrefix(42), roachpb.RKey([]byte{0x31, 'a', 0x00, 0x02})), "/Table/42/<util/encoding/encoding.go:382: unknown escape>"},
	}
	for i, test := range testCases {
		keyInfo := PrettyPrint(test.key)
		if test.exp != keyInfo {
			t.Fatalf("%d: expected %s, got %s", i, test.exp, keyInfo)
		}

		if test.exp != test.key.String() {
			t.Fatalf("%d: expected %s, got %s", i, test.exp, keyInfo)
		}
	}
}
