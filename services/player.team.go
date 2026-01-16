package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Faze-Technologies/go-utils/config"
)

type TeamAPIResponse struct {
	Success   bool        `json:"success"`
	ErrorCode interface{} `json:"error_code"`
	Message   string      `json:"message"`
	Data      []Team      `json:"data"`
}

type Team struct {
	ID              string `json:"_id"`
	Name            string `json:"name"`
	TeamAbbrevation string `json:"teamAbbrevation"`
	Logo            string `json:"logo"`
	TeamSlug        string `json:"teamSlug"`
	TeamColor       string `json:"teamColor"`
	TeamId          string `json:"teamId"`
}

type PlayerAPIResponse struct {
	Success   bool        `json:"success"`
	ErrorCode interface{} `json:"error_code"`
	Message   string      `json:"message"`
	Data      []Player    `json:"data"`
}

type Player struct {
	ID             string     `json:"_id"`
	Name           string     `json:"name"`
	JerseyNo       int        `json:"jerseyNo"`
	PlayerSlug     string     `json:"playerSlug"`
	Skills         string     `json:"skills"`
	EntityPlayerId int        `json:"entityPlayerId"`
	Team           PlayerTeam `json:"team"`
	PlayerQuality  string     `json:"playerQuality"`
	PlayerGender   string     `json:"playerGender"`
	PlayerId       string     `json:"playerId"`
	Dob            *time.Time `json:"dob,omitempty"`
	Country        string     `json:"country"`
	Alive          bool       `json:"alive,omitempty"`
	DateOfDeath    *time.Time `json:"dateOfDeath,omitempty"`
}

func (p *Player) UnmarshalJSON(data []byte) error {
	type Alias Player
	defaultAlive := true
	p.Alive = defaultAlive
	aux := &struct {
		Alive *bool `json:"alive,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Alive != nil {
		p.Alive = *aux.Alive
	}
	return nil
}

type PlayerTeam struct {
	ID              string `json:"_id"`
	Name            string `json:"name"`
	TeamAbbrevation string `json:"teamAbbrevation"`
	Logo            string `json:"logo"`
	TeamSlug        string `json:"teamSlug"`
	TeamId          string `json:"teamId"`
}

func FetchTeamsData(slugs []string) (map[string]Team, error) {
	if len(slugs) == 0 {
		return make(map[string]Team), nil
	}

	slugParam := strings.Join(slugs, ",")
	serviceUrl := config.GetServiceURL("teamService")
	url := fmt.Sprintf("%s/slugs/admin?slug=%s", serviceUrl, slugParam)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching teams: %v", err)
	}
	defer resp.Body.Close()

	var teamResponse TeamAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&teamResponse); err != nil {
		return nil, fmt.Errorf("error decoding teams response: %v", err)
	}

	if !teamResponse.Success && teamResponse.Message != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s", teamResponse.Message)
	}

	// Create map for easy lookup
	teamsMap := make(map[string]Team)
	for _, team := range teamResponse.Data {
		teamsMap[team.TeamSlug] = team
	}

	fmt.Printf("Successfully fetched %d teams\n", len(teamsMap))
	return teamsMap, nil
}

func FetchPlayersData(slugs []string) (map[string]Player, error) {
	if len(slugs) == 0 {
		return make(map[string]Player), nil
	}

	// Join all slugs with commas
	slugParam := strings.Join(slugs, ",")
	serviceUrl := config.GetServiceURL("playerService")
	url := fmt.Sprintf("%s/slugs/admin?slug=%s", serviceUrl, slugParam)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching players: %v", err)
	}
	defer resp.Body.Close()

	var playerResponse PlayerAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&playerResponse); err != nil {
		return nil, fmt.Errorf("error decoding players response: %v", err)
	}

	if !playerResponse.Success && playerResponse.Message != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s", playerResponse.Message)
	}

	// Create map for easy lookup
	playersMap := make(map[string]Player)
	for _, player := range playerResponse.Data {
		playersMap[player.PlayerSlug] = player
	}

	fmt.Printf("Successfully fetched %d players\n", len(playersMap))
	return playersMap, nil
}
