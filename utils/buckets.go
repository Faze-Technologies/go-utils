package utils

import (
	"fmt"

	"github.com/Faze-Technologies/go-utils/config"
)

// BucketType enumerates the recognized bucket-type keys in bucketsByEnv.
type BucketType string

const (
	BucketMoments     BucketType = "moments"
	BucketKYC         BucketType = "kyc"
	BucketContent     BucketType = "content"
	BucketContentApp  BucketType = "contentApp"
	BucketMarketplace BucketType = "marketplace"
	BucketNFTAssets   BucketType = "nftAssets"
)

// bucketsByEnv maps environment -> bucket type -> bucket name. One bucket per
// type, shared by every service that uses that type. dev isn't listed
// separately - GetBucket falls back to preprod's values for any environment
// other than "prod", since dev and preprod always use the same buckets.
//
// BucketContentApp (iap-admin-service) has no preprod/dev entry because none
// was given - GetBucket returns an error for it in those environments until
// one is added. BucketNFTAssets is a Cloudflare R2 bucket
// (superteam-event-admin-service), not GCS like the others.
var bucketsByEnv = map[string]map[BucketType]string{
	"prod": {
		BucketMoments:    "fc-moments-assests-prod-live",
		BucketContent:    "content-fancraze-com-live",
		BucketContentApp: "prod-content-faze-app-live",
		BucketKYC:        "prod-kyc-fancraze-com-live",
		BucketNFTAssets:  "fancraze-nft-assets",
	},
	"preprod": {
		BucketMoments:     "fc-moments-assests-prod",
		BucketKYC:         "prod-kyc-fancraze-com",
		BucketContent:     "content-fancraze-com",
		BucketContentApp:  "prod-content-faze-app",
		BucketMarketplace: "content-fancraze-com",
		BucketNFTAssets:   "fancraze-nft-assets_",
	},
	"dev": {
		BucketMoments:     "fc-moments-assests-prod",
		BucketKYC:         "prod-kyc-fancraze-com",
		BucketContent:     "content-fancraze-com",
		BucketContentApp:  "prod-content-faze-app",
		BucketMarketplace: "content-fancraze-com",
		BucketNFTAssets:   "fancraze-nft-assets_",
	},
}

// GetBucket returns the bucket name configured for bucketType in the current
// environment (config.GetString("environment"), i.e. dev/preprod/prod - see
// config.environmentProjects).
func GetBucket(bucketType BucketType) (string, error) {
	env := config.GetString("environment")

	bucket, ok := bucketsByEnv[env][bucketType]
	if !ok {
		return "", fmt.Errorf("no %s bucket for type %q", env, bucketType)
	}
	return bucket, nil
}
