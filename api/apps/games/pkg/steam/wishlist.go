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
	AppID int    `json:"appid"`
	Name  string `json:"name"`
}

// CapsuleURL is the 616x353 store capsule image of appID.
func CapsuleURL(appID int) string {
	return fmt.Sprintf(
		"https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/"+
			"%d/capsule_616x353.jpg",
		appID,
	)
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
		"data_request": map[string]any{},
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
