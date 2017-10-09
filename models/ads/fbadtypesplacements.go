package ads

type FacebookAdTypePlacements struct {
	// Types

	// Link Click Ads
	// Recommended image size: 1,200 x 628 pixels
	// Ad copy text: 90 characters
	// Headline: 25 characters
	// Link Description: 30 characters
	//
	// Supported placements:
	// Right Column
	// Desktop Newsfeed
	// Mobile Newsfeed
	// Audience Network
	// Instagram
	DoLinkClickAds bool `json:"doLinkClickAds"`

	// Video Ads
	// Ad copy text: 90 characters
	// Aspect ratios supported: 16:9 to 9:16
	// File size: up to 4 GB max
	// Continuous looping available
	// Video can be as long as 120 min., but most top-performing videos are 15-30 seconds
	//
	// Supported placements:
	// Desktop Newsfeed
	// Mobile Newsfeed
	// Audience Network
	// Instagram
	DoVideoAds bool `json:"doVideoAds"`

	// Boosted Page Posts
	// Recommended image size: 1,200 x 628 pixels
	// Ad copy text: unlimited
	// Headline: 25 characters
	// Link Description: 30 characters
	//
	// Supported placements:
	// Desktop Newsfeed
	// Mobile Newsfeed
	// Audience Network
	// Instagram
	DoBoostedPagePosts bool `json:"doBoostedPagePosts`

	// ... more here:
	// https://adespresso.com/guides/facebook-ads-beginner/facebook-ads-types/

	// Placements
	DoRightColumn     bool `json:"doRightColumn"`
	DoDesktopNewsfeed bool `json:"doDesktopNewsfeed"`
	DoMobileNewsfeed  bool `json:"doMobileNewsfeed"`
	DoAudienceNetwork bool `json:"doAudienceNetwork"`
	DoInstagram       bool `json:"doInstagram"`
}
