package jambase

import (
	"path"
	"runtime"
	"testing"
)

func TestJAMBase(t *testing.T) {
	_, ourPath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("couldn't find our path")
	}
	jhrPath := path.Join(path.Dir(ourPath), "testdata", "mystic", "general.jhr")
	jmb, err := NewJAMBase(jhrPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = jmb.ReadFixedHeader()
	if err != nil {
		t.Fatal(err)
	}
	msgChan, errChan := jmb.ReadMessages()
	select {
	case _, ok := <-msgChan:
		if !ok {
			return
		}
	case err, ok := <-errChan:
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return
		}
	}
}
