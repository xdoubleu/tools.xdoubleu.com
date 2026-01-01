package services

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"tools.xdoubleu.com/apps/goaltracker/internal/dtos"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
	"tools.xdoubleu.com/apps/goaltracker/internal/repositories"
	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
)

type GoalService struct {
	webURL    string
	goals     *repositories.GoalRepository
	states    *repositories.StateRepository
	progress  *repositories.ProgressRepository
	todoist   *TodoistService
	goodreads *GoodreadsService
	steam     *SteamService
}

type StateGoalsPair struct {
	State string
	Goals []models.Goal
}

func (service *GoalService) GetAllGoalsGroupedByStateAndParentGoal(
	ctx context.Context,
	userID string,
) ([]StateGoalsPair, error) {
	otherPeriod := "Outside Current Period"

	states, err := service.states.GetAll(ctx, userID)
	if err != nil {
		return nil, err
	}

	//nolint:exhaustruct //order is optional
	states = slices.Insert(states, 1, models.State{
		ID:   otherPeriod,
		Name: otherPeriod,
	})

	goals, err := service.goals.GetAll(ctx, userID)
	if err != nil {
		return nil, err
	}

	goalsMap := map[string][]models.Goal{}
	goalsMap[otherPeriod] = []models.Goal{}
	for _, goal := range goals {
		if goal.TypeID != nil && *goal.TypeID == models.BooksFromSpecificTag.ID {
			var progress *string
			progress, err = service.getProgressForSpecificTag(ctx, goal, userID)
			if err != nil {
				return nil, err
			}
			goal.Progress = progress
		}

		if goal.IsCurrentPeriod() {
			goalsMap[goal.StateID] = append(goalsMap[goal.StateID], goal)
		} else {
			goalsMap[otherPeriod] = append(goalsMap[otherPeriod], goal)
		}
	}

	result := []StateGoalsPair{}
	for _, state := range states {
		pair := StateGoalsPair{
			State: state.Name,
			Goals: goalsMap[state.ID],
		}

		if len(pair.Goals) == 0 {
			continue
		}

		result = append(result, pair)
	}

	return result, nil
}

func (service *GoalService) getProgressForSpecificTag(
	ctx context.Context,
	goal models.Goal,
	userID string,
) (*string, error) {
	progress := 0
	books, err := service.goodreads.GetBooksByTag(
		ctx,
		goal.Config["tag"],
		userID,
	)
	if err != nil {
		return nil, err
	}

	for _, book := range books {
		for _, dateRead := range book.DatesRead {
			if goal.PeriodStart().Before(dateRead) &&
				goal.PeriodEnd().After(dateRead) {
				progress++
				break
			}
		}
	}

	strProgress := strconv.Itoa(progress)
	return &strProgress, nil
}

func (service *GoalService) GetGoalByID(
	ctx context.Context,
	id string,
	userID string,
) (*models.Goal, error) {
	return service.goals.GetByID(ctx, id, userID)
}

func (service *GoalService) GetGoalsByTypeID(
	ctx context.Context,
	id int64,
	userID string,
) ([]models.Goal, error) {
	return service.goals.GetByTypeID(ctx, id, userID)
}

func (service *GoalService) ImportStatesFromTodoist(
	ctx context.Context,
	userID string,
) error {
	sections, err := service.todoist.GetSections(ctx)
	if err != nil {
		return err
	}

	sectionsMap := map[string]todoist.Section{}
	for _, section := range sections {
		sectionsMap[section.ID] = section
	}

	existingStates, err := service.states.GetAll(ctx, userID)
	if err != nil {
		return err
	}

	for _, state := range existingStates {
		_, ok := sectionsMap[state.ID]

		if ok {
			continue
		}

		err = service.states.Delete(ctx, &state, userID)
		if err != nil {
			return err
		}
	}

	for _, section := range sections {
		_, err = service.states.Upsert(
			ctx,
			section.ID,
			userID,
			section.Name,
			section.Order,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (service *GoalService) ImportGoalsFromTodoist(
	ctx context.Context,
	userID string,
) error {
	tasks, err := service.todoist.GetTasks(ctx)
	if err != nil {
		return err
	}

	tasksMap := map[string]todoist.Task{}
	for _, task := range tasks {
		tasksMap[task.ID] = task
	}

	existingGoals, err := service.goals.GetAll(ctx, userID)
	if err != nil {
		return err
	}

	for _, goal := range existingGoals {
		_, ok := tasksMap[goal.ID]

		if ok {
			continue
		}

		err = service.goals.Delete(ctx, &goal, userID)
		if err != nil {
			return err
		}
	}

	for _, task := range tasksMap {
		_, err = service.goals.Upsert(
			ctx,
			task.ID,
			userID,
			task.Content,
			task.SectionID,
			task.Due,
			task.Order,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (service *GoalService) LinkGoal(
	ctx context.Context,
	id string,
	userID string,
	linkGoalDto *dtos.LinkGoalDto,
) error {
	if linkGoalDto.Tag != nil && *linkGoalDto.Tag == "" {
		linkGoalDto.Tag = nil
	}

	goal, err := service.goals.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}

	return service.goals.Link(
		ctx,
		goal,
		userID,
		*linkGoalDto,
	)
}

func (service *GoalService) UnlinkGoal(
	ctx context.Context,
	id string,
	userID string,
) error {
	goal, err := service.goals.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}

	return service.goals.Unlink(
		ctx,
		*goal,
		userID,
	)
}

func (service *GoalService) CompleteGoal(
	ctx context.Context,
	id string,
	userID string,
) error {
	goal, err := service.goals.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}

	return service.todoist.CompleteTask(ctx, goal.ID)
}

func (service *GoalService) GetProgressByTypeIDAndDates(
	ctx context.Context,
	typeID int64,
	userID string,
	dateStart time.Time,
	dateEnd time.Time,
) ([]string, []string, error) {
	progresses, err := service.progress.GetByTypeIDAndDates(
		ctx,
		typeID,
		userID,
		dateStart,
		dateEnd,
	)
	if err != nil {
		return nil, nil, err
	}

	progressLabels := []string{}
	progressValues := []string{}

	for _, progress := range progresses {
		progressLabels = append(
			progressLabels,
			progress.Date.Format(models.ProgressDateFormat),
		)
		progressValues = append(progressValues, progress.Value)
	}

	return progressLabels, progressValues, nil
}

func (service *GoalService) SaveProgress(
	ctx context.Context,
	typeID int64,
	userID string,
	progressLabels []string,
	progressValues []string,
) error {
	return service.progress.Upsert(
		ctx,
		typeID,
		userID,
		progressLabels,
		progressValues,
	)
}

//nolint:gocognit //function is too complex
func (service *GoalService) GetListItemsByGoal(
	ctx context.Context,
	goal *models.Goal,
	userID string,
) ([]models.ListItem, error) {
	listItems := []models.ListItem{}
	periodStart := goal.PeriodStart()
	periodEnd := goal.PeriodEnd()

	switch *goal.TypeID {
	case models.BooksFromSpecificTag.ID:
		books, err := service.goodreads.GetBooksByTag(ctx, goal.Config["tag"], userID)
		if err != nil {
			return nil, err
		}

		for _, book := range books {
			var dateRead *time.Time
			for _, date := range book.DatesRead {
				if periodStart.Before(date) && periodEnd.After(date) {
					dateRead = &date
					break
				}
			}

			if dateRead == nil {
				continue
			}

			listItems = append(listItems, models.ListItem{
				ID:            book.ID,
				Value:         fmt.Sprintf("%s - %s", book.Title, book.Author),
				CompletedDate: *dateRead,
			})
		}
	case models.FinishedBooksThisYear.ID:
		books, err := service.goodreads.GetAllBooks(ctx, userID)
		if err != nil {
			return nil, err
		}

		for _, book := range books {
			var dateRead *time.Time
			for _, date := range book.DatesRead {
				if periodStart.Before(date) && periodEnd.After(date) {
					dateRead = &date
					break
				}
			}

			if dateRead == nil {
				continue
			}

			listItems = append(listItems, models.ListItem{
				ID:            book.ID,
				Value:         fmt.Sprintf("%s - %s", book.Title, book.Author),
				CompletedDate: *dateRead,
			})
		}
	case models.SteamCompletionRate.ID:
		games, err := service.steam.GetAllGames(ctx, userID)
		if err != nil {
			return nil, err
		}

		slices.SortFunc(games, func(a models.Game, b models.Game) int {
			fCRA, errA := strconv.ParseFloat(a.CompletionRate, 64)
			if errA != nil {
				return 0
			}
			fCRB, errB := strconv.ParseFloat(b.CompletionRate, 64)
			if errB != nil {
				return 0
			}

			if fCRA == fCRB {
				return 0
			}
			if fCRA < fCRB {
				return 1
			}
			return -1
		})

		for _, game := range games {
			//nolint:exhaustruct //other fields not applicable
			listItems = append(listItems, models.ListItem{
				ID: int64(game.ID),
				Value: fmt.Sprintf(
					"%s (%s%%) - %s%%",
					game.Name,
					game.CompletionRate,
					game.Contribution,
				),
			})
		}
	}

	return listItems, nil
}
