package libCallApi_test

import (
	"encoding/json"
	"testing"

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/response"
	"gotest.tools/v3/assert"
)

func TestCall(t *testing.T) {
	type Data struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	type Pagination struct {
		LastVisiblePage int  `json:"last_visible_page"`
		HasNextPage     bool `json:"has_next_page"`
	}
	type AnimeEpisodes struct {
		Data       []Data     `json:"data"`
		Pagination Pagination `json:"pagination"`
	}
	type TestCase struct {
		Name    string
		Request libCallApi.CallParam
		Result  *AnimeEpisodes
		Error   response.ErrorState
	}
	callParam := libCallApi.CallParamData{
		Api:        libCallApi.RemoteApi{Domain: "https://api.jikan.moe/v4/anime"},
		QueryStack: &[]string{"1/episodes", "200/episodes", "300/episodes", "400/episodes"},
	}
	testCases := []TestCase{
		{
			Name: "Step1",
			Result: &AnimeEpisodes{
				Data: []Data{
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/1", Title: "Asteroid Blues"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/2", Title: "Stray Dog Strut"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/3", Title: "Honky Tonk Women"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/4", Title: "Gateway Shuffle"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/5", Title: "Ballad of Fallen Angels"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/6", Title: "Sympathy for the Devil"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/7", Title: "Heavy Metal Queen"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/8", Title: "Waltz for Venus"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/9", Title: "Jamming with Edward"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/10", Title: "Ganymede Elegy"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/11", Title: "Toys in the Attic"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/12", Title: "Jupiter Jazz (Part 1)"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/13", Title: "Jupiter Jazz (Part 2)"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/14", Title: "Bohemian Rhapsody"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/15", Title: "My Funny Valentine"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/16", Title: "Black Dog Serenade"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/17", Title: "Mushroom Samba"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/18", Title: "Speak Like a Child"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/19", Title: "Wild Horses"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/20", Title: "Pierrot le Fou"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/21", Title: "Boogie Woogie Feng Shui"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/22", Title: "Cowboy Funk"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/23", Title: "Brain Scratch"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/24", Title: "Hard Luck Woman"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/25", Title: "The Real Folk Blues (Part 1)"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/26", Title: "The Real Folk Blues (Part 2)"},
				},
				Pagination: Pagination{
					LastVisiblePage: 1,
					HasNextPage:     false,
				},
			},
		},
		{
			Name: "Step2",
			Result: &AnimeEpisodes{
				Data: []Data{
					{Title: "Meeting at Full Speed − Is the Angel Male or Female?"},
					{Title: "What's Wrong? My Angel!"},
					{Title: "This Is the Man's Hand That Defeats Enemies with a Single Blow!"},
					{Title: "Angel Flown Down From Heaven..."},
					{Title: "I Love You! I Love You, Too!!"},
					{Title: "Let's Get Closer! Into That Girl's Mystery?!"},
					{Title: "I Will Become a Woman for You!!?"},
					{Title: "Such an Annoying Guy. Righteous Idiot, Kobayashi"},
					{Title: "It's the Magic Book!! I Don't Want Megu to Be a Guy!!"},
					{Title: "Beat It Up! The Little Demon's Curse!!"},
					{Title: "It's a Date, Megu, I'll Show You a Great Guy!"},
					{Title: "Of course! I'm Cursed After All!"},
					{Title: "Somehow Osaka!? The Dreaming Girl is a Big Thief!?"},
					{Title: "I Want to Die Between Your White Thighs!"},
					{Title: "We Can Meet the Old Wizard!"},
					{Title: "Don't Point Out to My Weaknesses! Fishing for Giant Crayfish"},
					{Title: "The Worst Person Has Come! Lady Keiko Appears!!"},
					{Title: "A Man’s Showdown! Samurai vs Average?"},
					{Title: "Megu vs Keiko! I Will Raise a Strong Man!!"},
					{Title: "Miki's Fiance!? I Will Protect You No Matter What!"},
					{Title: "Operation Break the Engagement! I Won't Give Miki to That Two-Faced Bastard!!"},
					{Title: "Prestige and Honor Be Damned! This Is the Power of Love!!"},
					{Title: "Yes, There Are Good Men! Mother's Emotional Interview Test!"},
					{Title: "Soga's Woman? I'll Crush Her with My Cute Legs"},
					{Title: "Genzou, Misogynist! What in the World Is That Scar on His Cheek?"},
					{Title: "Don't Embrace Me! Who Is Setsuka!?"},
					{Title: "It’s a Kiss!? My Spell Weakens!"},
					{Title: "I Will Go Back to Being a Boy!! It's a Man's Contest!!"},
					{Title: "You Molester Bastard, Beautiful Megu Will Sort You Out!"},
					{Title: "Is It Divine Punishment or a Curse? Well Done, Genzo!"},
					{Title: "Trapped Megu! Men Are All a Disturbing Bunch!"},
					{Title: "The Girl That Keiko Fell in Love With! M-me?"},
					{Title: "Can We Beat the Little Demon's Magic? Pervert Power!!"},
					{Title: "A Must-See − Megumi's Apron Look!! It's a Cooking Contest!"},
					{Title: "Genzo, You Cannot Escape! It's Seppuku for You!!"},
					{Title: "Megu’s Yukata Look, Why are You Running Away?"},
					{Title: "Yamato Nadeshiko Cup 1: Who's the Fated Partner? Fujiki Sees a Hawk."},
					{Title: "Yamato Nadeshiko Cup 2: What's the Big Deal About a Boxer's Punch? Megu Dances the Ring Magnificently!"},
					{Title: "Yamato Nadeshiko Cup 3: Megumi vs Keiko: It's a Ghost Battle!"},
					{Title: "Yamato Nadeshiko Cup 4: Yanagisawa's Counterattack. It's Okay, Because I'm Not a Woman."},
					{Title: "Yamato Nadeshiko Cup 5: Miki Gets Angry! What's That About Being a Man, You Stupid Megumi!"},
					{Title: "Yamato Nadeshiko Cup 6: I Am Strong, I'll Finish You Off with Fake Crying!"},
					{Title: "A Date with a Prince!? Megu Is a Princess!?"},
					{Title: "Kappa Kappa Fujiki! I Saw a Mermaid in the Midsummer Sea!"},
					{Title: "The Stone-Breaking Samurai, If You Want Miki, I’ll Defeat You!"},
					{Title: "It's the School Trip! It's a Confession! It's a Romantic Comedy Samurai?!"},
					{Title: "I‘m Miki's Prince! Hiyo, Silver!!"},
					{Title: `Gakusan's Game Begins! Fujiki's "Love" Explosion!!`},
					{Title: "To Reach the Heart of Captive Miki, Stone-Breaking Power!!"},
					{Title: "KISS! The Spell is Dispelled! Is the Angel a Boy or a Girl?"},
				},
				Pagination: Pagination{
					LastVisiblePage: 1,
					HasNextPage:     false,
				},
			},
		},
		{
			Name: "Step3",
			Result: &AnimeEpisodes{
				Data: []Data{
					{Title: "Transmigration"},
					{Title: "Yakumo"},
					{Title: "Sacrifice"},
					{Title: "Straying"},
				},
				Pagination: Pagination{
					LastVisiblePage: 1,
					HasNextPage:     false,
				},
			},
		},
		/*{
			Name: "Step4",
			Result: &AnimeEpisodes{
				Data: []Data{
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/1", Title: "Outlaw World"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/2", Title: "Star of Desire"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/3", Title: "Into Burning Space"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/4", Title: "When the Hot Ice Melts"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/5", Title: "Beast Girl, Ready to Pounce"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/6", Title: "Beautiful Assassin"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/7", Title: "Creeping Evil"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/8", Title: "Forced Departure"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/9", Title: "A Journey of Adventure...Huh?"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/10", Title: "Gathering for the Space Race"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/11", Title: "Adrift in Subspace"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/12", Title: "Mortal Combat with the El Dorado"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/13", Title: "Advance Guard from Another World"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/14", Title: "Final Countdown"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/15", Title: "The Seven Emerge"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/16", Title: "Between Life and Machine"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/17", Title: "The Strongest Woman in the Universe"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/18", Title: "Law and Lawlessness"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/19", Title: "Cats and Girls and Spaceships"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/20", Title: "Grave of the Dragon"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/21", Title: "Gravity Jailbreak"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/22", Title: "Cutting the Galactic Leyline"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/23", Title: "Maze of Despair"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/24", Title: "Return to Space"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/25", Title: "Maze of Despair"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/26", Title: "Return to Space"},
				},
				Pagination: Pagination{
					LastVisiblePage: 1,
					HasNextPage:     false,
				},
			},
		},*/
	}

	for id := range testCases {
		t.Run(
			testCases[id].Name, func(t *testing.T) {
				result := libCallApi.Call[AnimeEpisodes](&callParam)
				assert.DeepEqual(t, result.Resp, testCases[id].Result)
			},
		)
	}
}

type Data struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}
type Pagination struct {
	LastVisiblePage int  `json:"last_visible_page"`
	HasNextPage     bool `json:"has_next_page"`
}
type AnimeEpisodes struct {
	Data       []Data     `json:"data"`
	Pagination Pagination `json:"pagination"`
}

func (s AnimeEpisodes) SetStatus(a int) {

}
func (s AnimeEpisodes) SetHeaders(a map[string]string) {

}
func TestCallJSON(t *testing.T) {
	type TestCase struct {
		Name    string
		Request libCallApi.CallParam
		Result  *AnimeEpisodes
		Error   response.ErrorState
	}
	callParam := libCallApi.RemoteCallParamData[any, AnimeEpisodes]{
		Api:        libCallApi.RemoteApi{Domain: "https://api.jikan.moe/v4/anime"},
		QueryStack: &[]string{"1/episodes", "200/episodes", "300/episodes", "400/episodes"},
		Builder: func(status int, rawResp []byte, headers map[string]string) (*AnimeEpisodes, response.ErrorState) {
			var resp AnimeEpisodes
			err := json.Unmarshal(rawResp, &resp)
			if err != nil {
				return nil, response.ToError("", "", err)
			}
			return &resp, nil
		},
	}
	testCases := []TestCase{
		{
			Name: "Step1",
			Result: &AnimeEpisodes{
				Data: []Data{
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/1", Title: "Asteroid Blues"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/2", Title: "Stray Dog Strut"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/3", Title: "Honky Tonk Women"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/4", Title: "Gateway Shuffle"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/5", Title: "Ballad of Fallen Angels"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/6", Title: "Sympathy for the Devil"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/7", Title: "Heavy Metal Queen"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/8", Title: "Waltz for Venus"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/9", Title: "Jamming with Edward"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/10", Title: "Ganymede Elegy"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/11", Title: "Toys in the Attic"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/12", Title: "Jupiter Jazz (Part 1)"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/13", Title: "Jupiter Jazz (Part 2)"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/14", Title: "Bohemian Rhapsody"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/15", Title: "My Funny Valentine"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/16", Title: "Black Dog Serenade"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/17", Title: "Mushroom Samba"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/18", Title: "Speak Like a Child"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/19", Title: "Wild Horses"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/20", Title: "Pierrot le Fou"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/21", Title: "Boogie Woogie Feng Shui"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/22", Title: "Cowboy Funk"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/23", Title: "Brain Scratch"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/24", Title: "Hard Luck Woman"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/25", Title: "The Real Folk Blues (Part 1)"},
					{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/26", Title: "The Real Folk Blues (Part 2)"},
				},
				Pagination: Pagination{
					LastVisiblePage: 1,
					HasNextPage:     false,
				},
			},
		},
		{
			Name: "Step2",
			Result: &AnimeEpisodes{
				Data: []Data{
					{Title: "Meeting at Full Speed − Is the Angel Male or Female?"},
					{Title: "What's Wrong? My Angel!"},
					{Title: "This Is the Man's Hand That Defeats Enemies with a Single Blow!"},
					{Title: "Angel Flown Down From Heaven..."},
					{Title: "I Love You! I Love You, Too!!"},
					{Title: "Let's Get Closer! Into That Girl's Mystery?!"},
					{Title: "I Will Become a Woman for You!!?"},
					{Title: "Such an Annoying Guy. Righteous Idiot, Kobayashi"},
					{Title: "It's the Magic Book!! I Don't Want Megu to Be a Guy!!"},
					{Title: "Beat It Up! The Little Demon's Curse!!"},
					{Title: "It's a Date, Megu, I'll Show You a Great Guy!"},
					{Title: "Of course! I'm Cursed After All!"},
					{Title: "Somehow Osaka!? The Dreaming Girl is a Big Thief!?"},
					{Title: "I Want to Die Between Your White Thighs!"},
					{Title: "We Can Meet the Old Wizard!"},
					{Title: "Don't Point Out to My Weaknesses! Fishing for Giant Crayfish"},
					{Title: "The Worst Person Has Come! Lady Keiko Appears!!"},
					{Title: "A Man’s Showdown! Samurai vs Average?"},
					{Title: "Megu vs Keiko! I Will Raise a Strong Man!!"},
					{Title: "Miki's Fiance!? I Will Protect You No Matter What!"},
					{Title: "Operation Break the Engagement! I Won't Give Miki to That Two-Faced Bastard!!"},
					{Title: "Prestige and Honor Be Damned! This Is the Power of Love!!"},
					{Title: "Yes, There Are Good Men! Mother's Emotional Interview Test!"},
					{Title: "Soga's Woman? I'll Crush Her with My Cute Legs"},
					{Title: "Genzou, Misogynist! What in the World Is That Scar on His Cheek?"},
					{Title: "Don't Embrace Me! Who Is Setsuka!?"},
					{Title: "It’s a Kiss!? My Spell Weakens!"},
					{Title: "I Will Go Back to Being a Boy!! It's a Man's Contest!!"},
					{Title: "You Molester Bastard, Beautiful Megu Will Sort You Out!"},
					{Title: "Is It Divine Punishment or a Curse? Well Done, Genzo!"},
					{Title: "Trapped Megu! Men Are All a Disturbing Bunch!"},
					{Title: "The Girl That Keiko Fell in Love With! M-me?"},
					{Title: "Can We Beat the Little Demon's Magic? Pervert Power!!"},
					{Title: "A Must-See − Megumi's Apron Look!! It's a Cooking Contest!"},
					{Title: "Genzo, You Cannot Escape! It's Seppuku for You!!"},
					{Title: "Megu’s Yukata Look, Why are You Running Away?"},
					{Title: "Yamato Nadeshiko Cup 1: Who's the Fated Partner? Fujiki Sees a Hawk."},
					{Title: "Yamato Nadeshiko Cup 2: What's the Big Deal About a Boxer's Punch? Megu Dances the Ring Magnificently!"},
					{Title: "Yamato Nadeshiko Cup 3: Megumi vs Keiko: It's a Ghost Battle!"},
					{Title: "Yamato Nadeshiko Cup 4: Yanagisawa's Counterattack. It's Okay, Because I'm Not a Woman."},
					{Title: "Yamato Nadeshiko Cup 5: Miki Gets Angry! What's That About Being a Man, You Stupid Megumi!"},
					{Title: "Yamato Nadeshiko Cup 6: I Am Strong, I'll Finish You Off with Fake Crying!"},
					{Title: "A Date with a Prince!? Megu Is a Princess!?"},
					{Title: "Kappa Kappa Fujiki! I Saw a Mermaid in the Midsummer Sea!"},
					{Title: "The Stone-Breaking Samurai, If You Want Miki, I’ll Defeat You!"},
					{Title: "It's the School Trip! It's a Confession! It's a Romantic Comedy Samurai?!"},
					{Title: "I‘m Miki's Prince! Hiyo, Silver!!"},
					{Title: `Gakusan's Game Begins! Fujiki's "Love" Explosion!!`},
					{Title: "To Reach the Heart of Captive Miki, Stone-Breaking Power!!"},
					{Title: "KISS! The Spell is Dispelled! Is the Angel a Boy or a Girl?"},
				},
				Pagination: Pagination{
					LastVisiblePage: 1,
					HasNextPage:     false,
				},
			},
		},
		{
			Name: "Step3",
			Result: &AnimeEpisodes{
				Data: []Data{
					{Title: "Transmigration"},
					{Title: "Yakumo"},
					{Title: "Sacrifice"},
					{Title: "Straying"},
				},
				Pagination: Pagination{
					LastVisiblePage: 1,
					HasNextPage:     false,
				},
			},
		},
		/*{
			Name: "Step4",
			Result: &AnimeEpisodes{
				Data: []Data{
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/1", Title: "Outlaw World"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/2", Title: "Star of Desire"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/3", Title: "Into Burning Space"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/4", Title: "When the Hot Ice Melts"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/5", Title: "Beast Girl, Ready to Pounce"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/6", Title: "Beautiful Assassin"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/7", Title: "Creeping Evil"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/8", Title: "Forced Departure"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/9", Title: "A Journey of Adventure...Huh?"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/10", Title: "Gathering for the Space Race"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/11", Title: "Adrift in Subspace"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/12", Title: "Mortal Combat with the El Dorado"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/13", Title: "Advance Guard from Another World"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/14", Title: "Final Countdown"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/15", Title: "The Seven Emerge"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/16", Title: "Between Life and Machine"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/17", Title: "The Strongest Woman in the Universe"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/18", Title: "Law and Lawlessness"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/19", Title: "Cats and Girls and Spaceships"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/20", Title: "Grave of the Dragon"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/21", Title: "Gravity Jailbreak"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/22", Title: "Cutting the Galactic Leyline"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/23", Title: "Maze of Despair"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/24", Title: "Return to Space"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/25", Title: "Maze of Despair"},
					{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/26", Title: "Return to Space"},
				},
				Pagination: Pagination{
					LastVisiblePage: 1,
					HasNextPage:     false,
				},
			},
		},*/
	}

	for id := range testCases {
		t.Run(
			testCases[id].Name, func(t *testing.T) {
				result, err := libCallApi.RemoteCall[any, AnimeEpisodes](&callParam)
				assert.DeepEqual(t, err, testCases[id].Error)
				assert.DeepEqual(t, result, testCases[id].Result)
			},
		)
	}
}
