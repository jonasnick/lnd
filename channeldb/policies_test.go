package channeldb

import (
	"bytes"
	"reflect"
	"sort"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func makeFakePolicy() (*Policy, error) {
	fakePolicy := &Policy{
		Fee: 101,
	}

	preImg, err := randomBytes(32, 33)
	if err != nil {
		return nil, err
	}
	copy(fakePolicy.PaymentHash[:], preImg)

	return fakePolicy, nil
}

func makeRandomFakePolicy() (*Policy, error) {
	return makeFakePolicy()
}

func TestPolicySerialization(t *testing.T) {
	t.Parallel()

	fakePolicy, err := makeFakePolicy()
	if err != nil {
		t.Fatalf("unable to create policy: %v", err)
	}

	var b bytes.Buffer
	if err := serializePolicy(&b, fakePolicy); err != nil {
		t.Fatalf("unable to serialize outgoing policy: %v", err)
	}

	newPolicy, err := deserializePolicy(&b)
	if err != nil {
		t.Fatalf("unable to deserialize outgoing policy: %v", err)
	}

	if !reflect.DeepEqual(fakePolicy, newPolicy) {
		t.Fatalf("Policies do not match after "+
			"serialization/deserialization %v vs %v",
			spew.Sdump(fakePolicy),
			spew.Sdump(newPolicy),
		)
	}
}

func TestPolicyWorkflow(t *testing.T) {
	t.Parallel()

	db, cleanUp, err := makeTestDB()
	defer cleanUp()
	if err != nil {
		t.Fatalf("unable to make test db: %v", err)
	}

	fakePolicy, err := makeFakePolicy()
	if err != nil {
		t.Fatalf("unable to make fake policy: %v", err)
	}
	if err = db.AddPolicy(fakePolicy); err != nil {
		t.Fatalf("unable to put policy in DB: %v", err)
	}

	policies, err := db.FetchAllPolicies()
	if err != nil {
		t.Fatalf("unable to fetch policies from DB: %v", err)
	}

	expectedPolicies := []*Policy{fakePolicy}
	if !reflect.DeepEqual(policies, expectedPolicies) {
		t.Fatalf("Wrong policies after reading from DB."+
			"Got %v, want %v",
			spew.Sdump(policies),
			spew.Sdump(expectedPolicies),
		)
	}

	// Make some random policies
	for i := 0; i < 5; i++ {
		randomPolicy, err := makeRandomFakePolicy()
		if err != nil {
			t.Fatalf("Internal error in tests: %v", err)
		}

		if err = db.AddPolicy(randomPolicy); err != nil {
			t.Fatalf("unable to put policy in DB: %v", err)
		}

		expectedPolicies = append(expectedPolicies, randomPolicy)
	}

	policies, err = db.FetchAllPolicies()
	if err != nil {
		t.Fatalf("Can't get policies from DB: %v", err)
	}

	sort.Slice(policies, func(i, j int) bool {
		return bytes.Compare(policies[i].PaymentHash[:], policies[j].PaymentHash[:]) == -1
	})
	sort.Slice(expectedPolicies, func(i, j int) bool {
		return bytes.Compare(expectedPolicies[i].PaymentHash[:], expectedPolicies[j].PaymentHash[:]) == -1
	})
	if !reflect.DeepEqual(policies, expectedPolicies) {
		t.Fatalf("Wrong policies after reading from DB."+
			"Got %v, want %v",
			spew.Sdump(policies),
			spew.Sdump(expectedPolicies),
		)
	}

	for _, p := range policies {
		policy, err := db.LookupPolicy(p.PaymentHash)
		if err != nil {
			t.Fatalf("Can't look up policy in DB: %v", err)
		}
		if !reflect.DeepEqual(policy, p) {
			t.Fatalf("Wrong policies after reading from DB."+
				"Got %v, want %v",
				spew.Sdump(policy),
				spew.Sdump(p))
		}
	}

	// Delete all policies.
	if err = db.DeleteAllPolicies(); err != nil {
		t.Fatalf("unable to delete policies from DB: %v", err)
	}

	// Check that there is no policies after deletion
	policiesAfterDeletion, err := db.FetchAllPolicies()
	if err != nil {
		t.Fatalf("Can't get policies after deletion: %v", err)
	}
	if len(policiesAfterDeletion) != 0 {
		t.Fatalf("After deletion DB has %v policies, want %v",
			len(policiesAfterDeletion), 0)
	}
}
