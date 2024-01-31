package storage

import (
	"github.com/pflow-dev/go-metamodel/v2/codec"
	"github.com/pflow-dev/go-metamodel/v2/metamodel"
	. "github.com/pflow-dev/pflow-cli/internal/examples"
	"testing"
)

func TestUnzippingModel(t *testing.T) {
	mm := metamodel.New()
	url := "https://pflow.xyz/?z=" + DiningPhilosophers.Base64Zipped
	json, ok := mm.UnpackFromUrl(url, "ModelTable.json")
	if !ok {
		t.Errorf("Failed to unzip ModelTable")
	}
	t.Logf("json: %s", json)
}

func TestNewStorage(t *testing.T) {
	s := New(ResetDb("/tmp/pflow_test.db", true))
	for _, m := range ExampleModels {
		m.IpfsCid = codec.ToOid([]byte(m.Base64Zipped)).String()
		// if m.IpfsCid != newCid {
		// 	t.Errorf("Cid mismatch: %s %s", m.IpfsCid, newCid)
		// }

		id, err := s.Model.Create(
			m.Title,
			m.Description,
			m.Keywords,
			m.Base64Zipped,
			m.IpfsCid,
			"http://localhost:8083/p/",
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("inserted id: %v %v %v", m.Title, id, m.IpfsCid)

		found := s.Model.GetByCid(m.IpfsCid)
		if found.IpfsCid != m.IpfsCid {
			t.Errorf("Failed to find ModelTable by cid: %s", m.IpfsCid)
		}
	}
}

const (
	snippetUrl = "http://localhost:8083/sandbox/?z=UEsDBAoAAAAAABAFnVeLV9AO5wIAAOcCAAAOAAAAZGVjbGFyYXRpb24uanMgY29uc3QgZGVjbGFyYXRpb24gPSB7CiAgICAibW9kZWxUeXBlIjogInBldHJpTmV0IiwKICAgICJ2ZXJzaW9uIjogInYwIiwKICAgICJwbGFjZXMiOiB7CiAgICAgICAiZm9vIjogeyAib2Zmc2V0IjogMCwgIngiOiA0ODAsICJ5IjogMzIwLCAiaW5pdGlhbCI6IDEsICJjYXBhY2l0eSI6IDMgfQogICAgfSwKICAgICJ0cmFuc2l0aW9ucyI6IHsKICAgICAgICAgImJhciI6IHsgIngiOiA0MDAsICJ5IjogNDAwIH0sCiAgICAgICAgICJiYXoiOiB7ICJ4IjogNTYwLCAieSI6IDQwMCB9LAogICAgICAgICAiYWRkIjogeyAieCI6IDQwMCwgInkiOiAyNDAgfSwKICAgICAgICAgInN1YiI6IHsgIngiOiA1NjAsICJ5IjogMjQwIH0KICAgIH0sCiAgICAiYXJjcyI6IFsKICAgICAgICAgeyAic291cmNlIjogImFkZCIsICJ0YXJnZXQiOiAiZm9vIiwgIndlaWdodCI6IDEgfSwKICAgICAgICAgeyAic291cmNlIjogImZvbyIsICJ0YXJnZXQiOiAic3ViIiwgIndlaWdodCI6IDEgfSwKICAgICAgICAgeyAic291cmNlIjogImJhciIsICJ0YXJnZXQiOiAiZm9vIiwgIndlaWdodCI6IDMsICJpbmhpYml0IjogdHJ1ZSB9LAogICAgICAgICB7ICJzb3VyY2UiOiAiZm9vIiwgInRhcmdldCI6ICJiYXoiLCAid2VpZ2h0IjogMSwgImluaGliaXQiOiB0cnVlIH0KICAgIF0KfTsKLy8gUkVWSUVXOiBEU0wgZnVuY3Rpb24gYWxzbyBzdXBwb3J0ZWQKLy8gZnVuY3Rpb24gZGVjbGFyYXRpb24oe2ZuLCBjZWxsLCByb2xlfSkgeyB9ClBLAQIUAAoAAAAAABAFnVeLV9AO5wIAAOcCAAAOAAAAAAAAAAAAAAAAAAAAAABkZWNsYXJhdGlvbi5qc1BLBQYAAAAAAQABADwAAAATAwAAAAA="
)

func TestUnzipSnippet(t *testing.T) {
	t.Logf("snippetUrl: %s", snippetUrl)
	data, ok := metamodel.UnzipUrl(snippetUrl, "declaration.js")
	if !ok {
		t.Errorf("Failed to unzip SnippetTable")
	}
	newZip, ok := metamodel.ToEncodedZip([]byte(data), "declaration.js")
	data, ok = metamodel.UnzipUrl("?z="+newZip, "declaration.js")
	if !ok {
		t.Errorf("Failed to unzip SnippetTable")
	}
	t.Logf("data: %s", data)
	t.Logf("out: http://localhost:8083/sandbox/?z=%s", newZip)
	s := New(ResetDb("/tmp/pflow_test.db", true))
	newCid := codec.ToOid([]byte(data)).String()
	_, err := s.Snippet.Create(newCid, newZip, "title", "description", "keywords", "http://localhost:8083/sandbox/")
	if err != nil {
		t.Fatal("Failed to insert SnippetTable")
	}
	snippet := s.Snippet.GetByCid(newCid)
	if snippet.IpfsCid != newCid {
		t.Fatal("Failed to find SnippetTable by cid")
	}
	_, ok = metamodel.UnzipUrl("?z="+snippet.Base64Zipped, "declaration.js")
	if !ok {
		t.Errorf("Failed to unzip SnippetTable")
	}
}
