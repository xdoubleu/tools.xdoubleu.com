package goodreads

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"golang.org/x/net/html"
)

type client struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) Client {
	return client{
		logger: logger,
	}
}

func (client client) GetUserID(profileURL string) (*string, error) {
	c := colly.NewCollector()

	var userID string
	c.OnHTML(".profilePictureIcon", func(h *colly.HTMLElement) {
		imgURL := h.Attr("src")
		splittedSlash := strings.Split(imgURL, "/")
		userID = strings.Split(splittedSlash[len(splittedSlash)-1], ".jpg")[0]
	})

	err := c.Visit(profileURL)
	if err != nil {
		return nil, err
	}

	return &userID, nil
}

func (client client) GetBooks(userID string) ([]Book, error) {
	shelves, err := getAllShelves(userID)
	if err != nil {
		return nil, err
	}

	books := map[int64]*Book{}

	for _, shelf := range shelves {
		client.logger.Debug(fmt.Sprintf("fetching books on shelf %s", shelf))

		var booksOnShelf []Book
		booksOnShelf, err = getBooksForShelfOrTag(userID, &shelf, nil)
		if err != nil {
			return nil, err
		}

		for _, book := range booksOnShelf {
			books[book.ID] = &book
		}
	}

	tags, err := getAllTags(userID)
	if err != nil {
		return nil, err
	}

	for _, tag := range tags {
		client.logger.Debug(fmt.Sprintf("fetching books for tag %s", tag))

		var booksWithTag []Book
		booksWithTag, err = getBooksForShelfOrTag(userID, nil, &tag)
		if err != nil {
			return nil, err
		}

		for _, book := range booksWithTag {
			books[book.ID].Tags = append(books[book.ID].Tags, tag)
		}
	}

	booksSlice := []Book{}
	for _, book := range books {
		booksSlice = append(booksSlice, *book)
	}

	return booksSlice, nil
}

func getBooksForShelfOrTag(userID string, shelf *string, tag *string) ([]Book, error) {
	books := []Book{}

	page := 0
	for {
		page++

		var booksOnPage []Book
		booksOnPage, err := getBooksFromPage(userID, shelf, tag, page)
		if err != nil {
			return nil, err
		}

		if len(booksOnPage) == 0 {
			break
		}

		books = append(books, booksOnPage...)
	}

	return books, nil
}

func getAllShelves(userID string) ([]string, error) {
	shelves := []string{}

	c := colly.NewCollector()

	c.OnHTML("#paginatedShelfList", func(h *colly.HTMLElement) {
		children := h.DOM.Children().Nodes

		for _, child := range children {
			if isDivider(child) {
				// below the divider you can find tags, not shelves
				return
			}

			shelf := getShelfOrTagName(child)
			if shelf == nil {
				panic(errors.New("couldn't find shelf name"))
			}

			shelves = append(shelves, *shelf)
		}
	})

	err := c.Visit(
		fmt.Sprintf(
			"https://www.goodreads.com/review/list/%s",
			userID,
		),
	)
	if err != nil {
		return nil, err
	}

	return shelves, nil
}

func getAllTags(userID string) ([]string, error) {
	tags := []string{}

	c := colly.NewCollector()

	c.OnHTML("#paginatedShelfList", func(h *colly.HTMLElement) {
		children := h.DOM.Children().Nodes

		readingTags := false
		for _, child := range children {
			if isDivider(child) {
				// above the divider you can find shelves, not tags
				readingTags = true
				continue
			}

			if !readingTags {
				continue
			}

			tag := getShelfOrTagName(child)
			if tag == nil {
				panic(errors.New("couldn't find tag name"))
			}

			tags = append(tags, *tag)
		}
	})

	err := c.Visit(
		fmt.Sprintf(
			"https://www.goodreads.com/review/list/%s",
			userID,
		),
	)
	if err != nil {
		return nil, err
	}

	return tags, nil
}

func isDivider(child *html.Node) bool {
	for _, attr := range child.Attr {
		if attr.Key == "class" && attr.Val == "horizontalGreyDivider" {
			return true
		}
	}

	return false
}

func getShelfOrTagName(child *html.Node) *string {
	var innerChild *html.Node
	//nolint:lll //it is what it is
	for innerChild = child.FirstChild; innerChild != nil; innerChild = innerChild.NextSibling {
		if innerChild.Type == html.ElementNode {
			break
		}
	}

	for _, innerAttr := range innerChild.Attr {
		if innerAttr.Key != "href" {
			continue
		}

		return &strings.Split(strings.Split(innerAttr.Val, "?")[1], "=")[1]
	}

	return nil
}

func getBooksFromPage(
	userID string,
	shelf *string,
	tag *string,
	page int,
) ([]Book, error) {
	c := colly.NewCollector()

	books := []Book{}
	c.OnHTML(".bookalike.review", func(h *colly.HTMLElement) {
		titleElement := h.DOM.Find(".title .value a")
		url, ok := titleElement.Attr("href")
		if !ok {
			panic("no href attribute")
		}

		idStr := strings.Split(url, "/")[3]
		idStr = strings.Split(idStr, ".")[0]
		idStr = strings.Split(idStr, "-")[0]

		id, err := strconv.Atoi(idStr)
		if err != nil {
			panic(err)
		}

		book := Book{
			ID:        int64(id),
			Title:     titleElement.Text(),
			Author:    h.ChildText(".author .value a"),
			Shelf:     "",
			Tags:      []string{},
			DatesRead: getDatesRead(h),
		}

		if shelf != nil {
			book.Shelf = *shelf
		}

		if tag != nil {
			book.Tags = append(book.Tags, *tag)
		}

		books = append(books, book)
	})

	var shelfOrTag string
	if shelf != nil {
		shelfOrTag = *shelf
	}
	if tag != nil {
		shelfOrTag = *tag
	}

	err := c.Visit(
		fmt.Sprintf(
			"https://www.goodreads.com/review/list/%s?page=%d&shelf=%s",
			userID,
			page,
			shelfOrTag,
		),
	)
	if err != nil {
		time.Sleep(time.Second)
		return getBooksFromPage(userID, shelf, tag, page)
	}

	return books, nil
}

func getDatesRead(h *colly.HTMLElement) []time.Time {
	result := []time.Time{}
	var err error

	dateReadStrs := h.ChildTexts(".date_read .value span")
	for _, dateReadStr := range dateReadStrs {
		if dateReadStr == "not set" {
			continue
		}

		possibleDateFormats := []string{
			"Jan 02, 2006",
			"Jan 2006",
		}

		var dateRead time.Time
		for _, dateFormat := range possibleDateFormats {
			dateRead, err = time.Parse(dateFormat, dateReadStr)
			if err == nil {
				break
			}
		}

		if err != nil {
			panic(err)
		}

		result = append(result, dateRead)
	}

	return result
}
