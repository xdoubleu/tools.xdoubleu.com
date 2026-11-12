package steam

import (
	"context"
	"encoding/json"
	"fmt"
)

// WishlistItem is one wishlisted app; DateAdded is Unix seconds.
type WishlistItem struct {
	AppID     int   `json:"appid"`
	Priority  int   `json:"priority"`
	DateAdded int64 `json:"date_added"`
}

type WishlistResponse struct {
	Response struct {
		Items []WishlistItem `json:"items"`
	} `json:"response"`
}

func (client client) GetWishlist(
	ctx context.Context,
	steamID string,
) (*WishlistResponse, error) {
	var resp WishlistResponse

	err := client.sendRequestAPI(
		ctx,
		"IWishlistService/GetWishlist/v1",
		fmt.Sprintf("steamid=%s", steamID),
		&resp,
	)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// StoreApp is an app resolved through the store browse API.
type StoreApp struct {
	AppID  int         `json:"appid"`
	Name   string      `json:"name"`
	Assets StoreAssets `json:"assets"`
}

// StoreAssets holds an app's image paths, relative to its asset directory.
type StoreAssets struct {
	// MainCapsule is "<hash>/capsule_616x353.jpg".
	MainCapsule string `json:"main_capsule"`
}

const capsuleBase = "https://shared.cloudflare.steamstatic.com/" +
	"store_item_assets/steam/apps/"

// CapsuleURL is the 616x353 store capsule image of appID. Older apps serve it
// at this unhashed path; newer ones only under a hash (see StoreApp.ImageURL).
func CapsuleURL(appID int) string {
	return fmt.Sprintf("%s%d/capsule_616x353.jpg", capsuleBase, appID)
}

// ImageURL is the app's capsule, hashed when the store reports a hash.
func (a StoreApp) ImageURL() string {
	if a.Assets.MainCapsule == "" {
		return CapsuleURL(a.AppID)
	}
	return fmt.Sprintf("%s%d/%s", capsuleBase, a.AppID, a.Assets.MainCapsule)
}

type storeItemsResponse struct {
	Response struct {
		StoreItems []StoreApp `json:"store_items"`
	} `json:"response"`
}

func (client client) GetStoreItems(
	ctx context.Context,
	appIDs []int,
) ([]StoreApp, error) {
	if len(appIDs) == 0 {
		return nil, nil
	}

	ids := make([]map[string]int, len(appIDs))
	for i, id := range appIDs {
		ids[i] = map[string]int{"appid": id}
	}
	input, err := json.Marshal(map[string]any{
		"ids":          ids,
		"context":      map[string]string{"language": "english", "country_code": "BE"},
		"data_request": map[string]any{"include_assets": true},
	})
	if err != nil {
		return nil, err
	}

	var resp storeItemsResponse
	err = client.sendRequestAPI(
		ctx,
		"IStoreBrowseService/GetItems/v1",
		"input_json="+string(input),
		&resp,
	)
	if err != nil {
		return nil, err
	}

	return resp.Response.StoreItems, nil
}
