package kube

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperfake "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/fake"
	fakeskupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1/fake"
	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestCmdLinkStatus_ValidateInput(t *testing.T) {
	type test struct {
		name                string
		args                []string
		flags               common.CommandLinkStatusFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectedError       string
	}

	testTable := []test{
		{
			name:                "missing CRD",
			args:                []string{"my-connector", "8080"},
			flags:               common.CommandLinkStatusFlags{},
			skupperErrorMessage: utils.CrdErr,
			expectedError:       utils.CrdHelpErr,
		},
		{
			name: "more than one argument was specified",
			args: []string{"my-link", ""},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedError: "this command only accepts one argument",
		},
		{
			name:          "there are no sites",
			args:          []string{},
			expectedError: "there is no skupper site available",
		},
		{
			name:  "output format is not valid",
			args:  []string{"my-link"},
			flags: common.CommandLinkStatusFlags{Output: "not-valid"},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedError: "output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdLinkStatusWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdLinkStatus_InputToOptions(t *testing.T) {

	t.Run("input to options", func(t *testing.T) {

		cmd, err := newCmdLinkStatusWithMocks("test", nil, nil, "")
		assert.Assert(t, err)

		cmd.Flags = &common.CommandLinkStatusFlags{
			Output: "json",
		}

		cmd.InputToOptions()

		assert.Check(t, cmd.output == "json")

	})

}

func TestCmdLinkStatus_Run(t *testing.T) {
	type test struct {
		name                string
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
		linkName            string
		output              string
	}

	testTable := []test{
		{
			name: "runs ok showing all the links",
			skupperObjects: []runtime.Object{
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
		},
		{
			name: "runs ok showing all the links in yaml format",
			skupperObjects: []runtime.Object{
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			output: "yaml",
		},
		{
			name: "runs ok showing one of the links",
			skupperObjects: []runtime.Object{
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			linkName: "link2",
		},
		{
			name: "runs ok showing one of the links in json format",
			skupperObjects: []runtime.Object{
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			linkName: "link2",
			output:   "json",
		},
		{
			name:                "run fails",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
		{
			name: "runs ok but there are no links",
		},
		{
			name: "there is no link with the name specified as an argument",
			skupperObjects: []runtime.Object{
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			linkName:     "link3",
			errorMessage: "links.skupper.io \"link3\" not found",
		},
		{
			name: "fails showing all the links in yaml format",
			skupperObjects: []runtime.Object{
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			output:       "unsupported",
			errorMessage: "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdLinkStatusWithMocks("test", nil, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)
		cmd.linkName = test.linkName
		cmd.output = test.output

		t.Run(test.name, func(t *testing.T) {

			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error(), err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func TestCmdLinkStatus_WaitUntilReady(t *testing.T) {

	t.Run("wait until ready", func(t *testing.T) {

		cmd, err := newCmdLinkStatusWithMocks("test", nil, nil, "")
		assert.Assert(t, err)

		result := cmd.WaitUntil()
		assert.Check(t, result == nil)

	})

}

// Verifies that multiple links are output in deterministic order (sorted by name).
// It uses a reactor that returns links in unsorted order [z, a, m]
func TestCmdLinkStatus_LinkListSortedByName(t *testing.T) {
	linkA := &v2alpha1.Link{ObjectMeta: v1.ObjectMeta{Name: "a-link", Namespace: "test"}}
	linkM := &v2alpha1.Link{ObjectMeta: v1.ObjectMeta{Name: "m-link", Namespace: "test"}}
	linkZ := &v2alpha1.Link{ObjectMeta: v1.ObjectMeta{Name: "z-link", Namespace: "test"}}

	skupperObjects := []runtime.Object{
		&v2alpha1.SiteList{
			Items: []v2alpha1.Site{
				{ObjectMeta: v1.ObjectMeta{Name: "the-site", Namespace: "test"}},
			},
		},
		linkZ, linkA, linkM,
	}
	clientset := skupperfake.NewSimpleClientset(skupperObjects...)
	fakeTyped := clientset.SkupperV2alpha1().(*fakeskupperv2alpha1.FakeSkupperV2alpha1)
	fakeTyped.PrependReactor("list", "links", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v2alpha1.LinkList{Items: []v2alpha1.Link{*linkZ, *linkA, *linkM}}, nil
	})

	kubeClient := &client.KubeClient{Namespace: "test", Skupper: clientset}
	cmd := &CmdLinkStatus{Client: kubeClient.GetSkupperClient().SkupperV2alpha1(), Namespace: "test"}
	cmd.output = "json"

	r, w, err := os.Pipe()
	assert.NilError(t, err)
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = oldStdout; w.Close() }()

	err = cmd.Run()
	assert.NilError(t, err)
	w.Close()
	data, err := io.ReadAll(r)
	assert.NilError(t, err)
	dec := json.NewDecoder(strings.NewReader(string(data)))
	var names []string
	for {
		var link v2alpha1.Link
		if err := dec.Decode(&link); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		names = append(names, link.Name)
	}
	assert.DeepEqual(t, names, []string{"a-link", "m-link", "z-link"})
}
// --- helper methods

func newCmdLinkStatusWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdLinkStatus, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdLinkStatus := &CmdLinkStatus{
		Client:    client.GetSkupperClient().SkupperV2alpha1(),
		Namespace: namespace,
	}

	return cmdLinkStatus, nil
}
