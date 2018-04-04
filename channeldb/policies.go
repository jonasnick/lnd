package channeldb

import (
	"bytes"
	"io"

	"github.com/coreos/bbolt"
	"github.com/lightningnetwork/lnd/lnwire"
)

var (
	// policyBucket is the name of the bucket within the database that
	// stores all data related to policies.
	policyBucket = []byte("policies")
)

type Policy struct {
	PaymentHash [32]byte
	Fee         lnwire.MilliSatoshi
}

func (db *DB) AddPolicy(policy *Policy) error {
	// We first serialize the payment before starting the database
	// transaction so we can avoid creating a DB payment in the case of a
	// serialization error.
	var b bytes.Buffer
	if err := serializePolicy(&b, policy); err != nil {
		return err
	}
	policyBytes := b.Bytes()

	return db.Batch(func(tx *bolt.Tx) error {
		policies, err := tx.CreateBucketIfNotExists(policyBucket)
		if err != nil {
			return err
		}

		return policies.Put(policy.PaymentHash[:], policyBytes)
	})
}

// FetchAllPayments returns all outgoing payments in DB.
func (db *DB) FetchAllPolicies() ([]*Policy, error) {
	var policies []*Policy

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(policyBucket)
		if bucket == nil {
			return ErrNoPaymentsCreated
		}

		return bucket.ForEach(func(k, v []byte) error {
			// If the value is nil, then we ignore it as it may be
			// a sub-bucket.
			if v == nil {
				return nil
			}

			r := bytes.NewReader(v)
			policy, err := deserializePolicy(r)
			if err != nil {
				return err
			}

			policies = append(policies, policy)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return policies, nil
}

// DeleteAllPayments deletes all policies from DB.
func (db *DB) DeleteAllPolicies() error {
	return db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(policyBucket)
		if err != nil && err != bolt.ErrBucketNotFound {
			return err
		}

		_, err = tx.CreateBucket(policyBucket)
		return err
	})
}

func fetchPolicy(policyNum []byte, policies *bolt.Bucket) (*Policy, error) {
	policyBytes := policies.Get(policyNum)
	if policyBytes == nil {
		return nil, ErrPolicyNotFound
	}

	policyReader := bytes.NewReader(policyBytes)

	return deserializePolicy(policyReader)
}

func (d *DB) LookupPolicy(paymentHash [32]byte) (*Policy, error) {
	var policy *Policy
	err := d.View(func(tx *bolt.Tx) error {
		policies := tx.Bucket(policyBucket)
		if policies == nil {
			return ErrNoPoliciesCreated
		}

		// Check the policy index to see if an policy paying to this
		// hash exists within the DB.
		policyBytes := policies.Get(paymentHash[:])
		if policyBytes == nil {
			return ErrPolicyNotFound
		}

		policyReader := bytes.NewReader(policyBytes)
		apolicy, err := deserializePolicy(policyReader)
		if err != nil {
			return err
		}
		policy = apolicy

		return nil
	})
	if err != nil {
		return nil, err
	}

	return policy, nil
}

func serializePolicy(w io.Writer, p *Policy) error {
	var scratch [8]byte

	if _, err := w.Write(p.PaymentHash[:]); err != nil {
		return err
	}

	byteOrder.PutUint64(scratch[:], uint64(p.Fee))
	if _, err := w.Write(scratch[:]); err != nil {
		return err
	}

	return nil
}

func deserializePolicy(r io.Reader) (*Policy, error) {
	var scratch [8]byte

	p := &Policy{}

	if _, err := r.Read(p.PaymentHash[:]); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(r, scratch[:]); err != nil {
		return nil, err
	}
	p.Fee = lnwire.MilliSatoshi(byteOrder.Uint64(scratch[:]))

	return p, nil
}
