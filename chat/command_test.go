package chat

import (
	"fmt"
	"testing"

	"github.com/shazow/ssh-chat/chat/message"
	"github.com/shazow/ssh-chat/set"
)

func TestAwayCommands(t *testing.T) {
	cmds := &Commands{}
	InitCommands(cmds)

	room := NewRoom()
	go room.Serve()
	defer room.Close()

	// steps are order dependent
	// User can be "away" or "not away" using 3 commands "/away [msg]", "/away", "/back"
	// 2^3 possible cases, run all and verify state at the end
	type step struct {
		// input
		Msg string

		// expected output
		IsUserAway  bool
		AwayMessage string

		// expected state change
		ExpectsError func(awayBefore bool) bool
	}
	neverError := func(_ bool) bool { return false }
	// if the user was away before, then the error is expected
	errorIfAwayBefore := func(awayBefore bool) bool { return awayBefore }

	awayStep := step{"/away snorkling", true, "snorkling", neverError}
	notAwayStep := step{"/away", false, "", errorIfAwayBefore}
	backStep := step{"/back", false, "", errorIfAwayBefore}

	steps := []step{awayStep, notAwayStep, backStep}
	cases := [][]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("Case: %d, %d, %d", c[0], c[1], c[2]), func(t *testing.T) {

			u := message.NewUser(message.SimpleID("shark"))

			for _, s := range []step{steps[c[0]], steps[c[1]], steps[c[2]]} {
				msg, _ := message.NewPublicMsg(s.Msg, u).ParseCommand()

				awayBeforeCommand, _, _ := u.GetAway()

				err := cmds.Run(room, *msg)
				if err != nil && s.ExpectsError(awayBeforeCommand) {
					t.Fatalf("unexpected error running the command: %+v", err)
				}

				isAway, _, awayMsg := u.GetAway()
				if isAway != s.IsUserAway {
					t.Fatalf("expected user away state '%t' not equals to actual '%t' after message '%s'", s.IsUserAway, isAway, s.Msg)
				}
				if awayMsg != s.AwayMessage {
					t.Fatalf("expected user away message '%s' not equal to actual '%s' after message '%s'", s.AwayMessage, awayMsg, s.Msg)
				}
			}

		})
	}
}

func runRoomCommand(t *testing.T, room *Room, user *message.User, input, want string) {
	t.Helper()
	cmd, _ := message.NewPublicMsg(input, user).ParseCommand()
	err := room.commands.Run(room, *cmd)
	if err != nil {
		if got := "Err: " + err.Error(); got != want {
			t.Fatalf("%s: got %q, want %q", input, got, want)
		}
		return
	}
	select {
	case m := <-room.broadcast:
		if reply, ok := m.(message.MessageTo); !ok || reply.To() != user {
			t.Fatalf("%s: reply not directed to requester: %v", input, m)
		}
		if got := m.String(); got != "-> "+want {
			t.Fatalf("%s: got %q, want %q", input, got, "-> "+want)
		}
	default:
		t.Fatalf("%s: missing reply", input)
	}
}

func TestHistoryCommands(t *testing.T) {
	room := NewRoomWithHistory(4)
	defer room.Close()
	alice := message.NewUser(message.SimpleID("alice"))
	bob := message.NewUser(message.SimpleID("bob"))
	for _, user := range []*message.User{alice, bob} {
		if _, err := room.Join(user); err != nil {
			t.Fatal(err)
		}
		<-room.broadcast
	}

	room.HandleMsg(message.NewPublicMsg("evicted", bob))
	room.HandleMsg(message.NewPublicMsg("Good Morning", bob))
	room.HandleMsg(message.NewAnnounceMsg("server morning notice"))
	room.HandleMsg(message.NewEmoteMsg("waves", bob))
	room.HandleMsg(message.NewPublicMsg("good morning again", alice))
	private := message.NewPrivateMsg("morning secret", alice, bob)
	bob.ConsumeOne()
	room.HandleMsg(&private)
	alice.ConsumeOne()
	room.HandleMsg(message.NewSystemMsg("morning secret", alice))

	runRoomCommand(t, room, alice, "/history", "Recent public messages (3):\r\nbob: Good Morning\r\n** bob waves\r\nalice: good morning again")
	runRoomCommand(t, room, alice, "/history 1", "Recent public messages (1):\r\nalice: good morning again")
	runRoomCommand(t, room, alice, "/history 101", "Err: history count must be between 1 and 100")
	runRoomCommand(t, room, alice, "/history 0", "Err: history count must be between 1 and 100")
	runRoomCommand(t, room, alice, "/history 2 3", "Err: usage: /history [COUNT]")
	runRoomCommand(t, room, alice, "/search GOOD MoRNING", "Search results (2):\r\nbob: Good Morning\r\nalice: good morning again")
	runRoomCommand(t, room, alice, "/search secret", "No public messages found.")
	runRoomCommand(t, room, alice, "/search", "Err: usage: /search TEXT")

	if err := alice.Focused.Add(set.Itemize("carol", set.ZeroValue)); err != nil {
		t.Fatal(err)
	}
	runRoomCommand(t, room, alice, "/history", "Recent public messages (2):\r\n** bob waves\r\nalice: good morning again")
	if err := alice.Ignored.Add(set.Itemize("bob", set.ZeroValue)); err != nil {
		t.Fatal(err)
	}
	runRoomCommand(t, room, alice, "/history", "Recent public messages (1):\r\nalice: good morning again")
}

func TestTopicAndBellCommands(t *testing.T) {
	room := NewRoom()
	defer room.Close()
	user := message.NewUser(message.SimpleID("tester"))
	member, err := room.Join(user)
	if err != nil {
		t.Fatal(err)
	}
	<-room.broadcast

	runRoomCommand(t, room, user, "/topic", "No room topic is set.")
	runRoomCommand(t, room, user, "/topic a test", "Err: must be op to change the room topic")
	member.IsOp = true
	runRoomCommand(t, room, user, "/topic a test", "Room topic: a test")
	member.IsOp = false
	runRoomCommand(t, room, user, "/topic", "Room topic: a test")
	member.IsOp = true
	runRoomCommand(t, room, user, "/topic -", "Room topic cleared.")
	runRoomCommand(t, room, user, "/topic", "No room topic is set.")

	runRoomCommand(t, room, user, "/bell off", "Terminal bell: off")
	if user.Config().Bell {
		t.Fatal("expected bell to be off")
	}
	runRoomCommand(t, room, user, "/bell", "Terminal bell: on")
	runRoomCommand(t, room, user, "/bell on", "Terminal bell: on")
	runRoomCommand(t, room, user, "/bell invalid", "Err: usage: /bell [on|off]")
	if !user.Config().Bell {
		t.Fatal("expected bell to remain on")
	}
}
