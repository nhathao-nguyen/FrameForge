package scheduler

import "testing"

func TestResolveFrontierIsDeterministicAndHandlesJoin(t *testing.T) {
	nodes := []Node{{Key: "join", Dependencies: []string{"a", "b"}}, {Key: "b"}, {Key: "a"}, {Key: "later", Dependencies: []string{"join"}}}
	frontier, err := ResolveFrontier(nodes, []Step{{Key: "a", Status: "completed"}, {Key: "b", Status: "skipped"}, {Key: "join", Status: "pending"}, {Key: "later", Status: "pending"}}, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(frontier.Ready) != 1 || frontier.Ready[0] != "join" || len(frontier.Blocked) != 0 {
		t.Fatalf("unexpected frontier: %+v", frontier)
	}
}

func TestResolveFrontierBlocksFailedDependencyAndBoundsOutput(t *testing.T) {
	nodes := []Node{{Key: "z", Dependencies: []string{"bad"}}, {Key: "bad"}, {Key: "a"}, {Key: "b"}, {Key: "c"}}
	frontier, err := ResolveFrontier(nodes, []Step{{Key: "bad", Status: "failed"}, {Key: "z", Status: "pending"}, {Key: "a", Status: "pending"}, {Key: "b", Status: "pending"}, {Key: "c", Status: "pending"}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(frontier.Ready) != 2 || len(frontier.Blocked) != 1 || frontier.Blocked[0] != "z" {
		t.Fatalf("unexpected bounded frontier: %+v", frontier)
	}
}

func TestResolveFrontierRejectsInvalidGraphs(t *testing.T) {
	cases := [][]Node{
		{{Key: "a", Dependencies: []string{"a"}}},
		{{Key: "a", Dependencies: []string{"missing"}}},
		{{Key: "a", Dependencies: []string{"b"}}, {Key: "b", Dependencies: []string{"a"}}},
		{{Key: "a"}, {Key: "a"}},
	}
	for _, nodes := range cases {
		if _, err := ResolveFrontier(nodes, []Step{{Key: "a", Status: "pending"}, {Key: "b", Status: "pending"}}, 1); err == nil {
			t.Errorf("expected invalid graph error for %+v", nodes)
		}
	}
}
