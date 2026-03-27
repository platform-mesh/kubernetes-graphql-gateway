package authn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jellydator/ttlcache/v3"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Validator validates bearer tokens.
type Validator interface {
	Validate(ctx context.Context, token string) (bool, error)
}

const maxCacheSize = 10000

// TokenReviewValidator validates tokens via the Kubernetes TokenReview API.
type TokenReviewValidator struct {
	clientset kubernetes.Interface
	cache     *ttlcache.Cache[string, bool]
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func newCache(ttl time.Duration) *ttlcache.Cache[string, bool] {
	if ttl <= 0 {
		return nil
	}
	return ttlcache.New(
		ttlcache.WithTTL[string, bool](ttl),
		ttlcache.WithCapacity[string, bool](maxCacheSize),
	)
}

// NewTokenReviewValidator creates a validator that calls TokenReview on the
// given cluster. If cacheTTL <= 0, caching is disabled and every request
// triggers an API call.
func NewTokenReviewValidator(cfg *rest.Config, cacheTTL time.Duration) (*TokenReviewValidator, error) {
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &TokenReviewValidator{
		clientset: cs,
		cache:     newCache(cacheTTL),
	}, nil
}

// NewTokenReviewValidatorFromClientset creates a validator from an existing
// clientset — useful for testing.
func NewTokenReviewValidatorFromClientset(cs kubernetes.Interface, cacheTTL time.Duration) *TokenReviewValidator {
	return &TokenReviewValidator{
		clientset: cs,
		cache:     newCache(cacheTTL),
	}
}

func (v *TokenReviewValidator) Validate(ctx context.Context, token string) (bool, error) {
	key := hashToken(token)
	if v.cache != nil {
		if item := v.cache.Get(key); item != nil {
			return item.Value(), nil
		}
	}

	tr, err := v.clientset.AuthenticationV1().TokenReviews().Create(ctx, &authenticationv1.TokenReview{
		Spec: authenticationv1.TokenReviewSpec{Token: token},
	}, metav1.CreateOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "TokenReview API call failed")
		return false, err
	}

	// TODO: use min(cacheTTL, tokenExpiry) to avoid serving expired tokens from cache.
	if v.cache != nil {
		v.cache.Set(key, tr.Status.Authenticated, ttlcache.DefaultTTL)
	}
	return tr.Status.Authenticated, nil
}

// Start begins automatic cache cleanup. Blocks until ctx is cancelled.
func (v *TokenReviewValidator) Start(ctx context.Context) {
	if v.cache == nil {
		return
	}
	go v.cache.Start()
	<-ctx.Done()
	v.cache.Stop()
}
