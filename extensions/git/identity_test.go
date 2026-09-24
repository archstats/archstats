package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func commitBy(name, email string) *rawCommit { return &rawCommit{AuthorName: name, AuthorEmail: email} }

func TestOneEmailIsOnePerson(t *testing.T) {
	cs := []*rawCommit{
		commitBy("Jeff Fischer", "jfischer@blc.org"), commitBy("Jeff Fischer", "jfischer@blc.org"),
		commitBy("jefffischer", "JFischer@blc.org"),
	}
	canonicalizeAuthors(cs)
	for _, c := range cs {
		assert.Equal(t, "Jeff Fischer", c.AuthorName)
	}
}

func TestAFullNameJoinsEmails(t *testing.T) {
	cs := []*rawCommit{commitBy("Jon Fleschler", "jon@gmail.com"), commitBy("Jon Fleschler", "jon@work.com"), commitBy("jfleschler", "jon@work.com")}
	canonicalizeAuthors(cs)
	for _, c := range cs {
		assert.Equal(t, "Jon Fleschler", c.AuthorName)
	}
}

func TestABareHandleDoesNotJoinStrangers(t *testing.T) {
	cs := []*rawCommit{commitBy("alex", "a@one.com"), commitBy("alex", "b@two.com")}
	canonicalizeAuthors(cs)
	assert.Equal(t, "alex", cs[0].AuthorName)
	assert.Equal(t, "alex", cs[1].AuthorName)
	// Still two identities: nothing links a@one.com to b@two.com.
	cs[0].AuthorName = "x"
	assert.NotEqual(t, cs[0].AuthorName, cs[1].AuthorName)
}

func TestPlaceholderEmailsNeverMerge(t *testing.T) {
	cs := []*rawCommit{commitBy("AndreyMaz", "none@none"), commitBy("swon", "none@none")}
	canonicalizeAuthors(cs)
	assert.Equal(t, "AndreyMaz", cs[0].AuthorName)
	assert.Equal(t, "swon", cs[1].AuthorName)
}

// Commits made through GitHub's web UI carry a no-reply address and the
// login, which is often the full name without its spaces. Broadleaf listed
// "Stanislav Fedorov" and "StanislavFedorov" as two people.
func TestAGitHubLoginJoinsTheFullNameItSpells(t *testing.T) {
	cs := []*rawCommit{
		commitBy("Stanislav Fedorov", "sfedorov@blc.com"), commitBy("Stanislav Fedorov", "sfedorov@blc.com"),
		commitBy("StanislavFedorov", "42337700+StanislavFedorov@users.noreply.github.com"),
		// A typo in the name under another address is not provably the same person.
		commitBy("Elbert Bautiste", "ebautiste@blc.org"), commitBy("Elbert Bautista", "ebautista@blc.org"),
	}
	canonicalizeAuthors(cs)
	assert.Equal(t, "Stanislav Fedorov", cs[2].AuthorName)
	assert.Equal(t, "Elbert Bautiste", cs[3].AuthorName)
}

func TestNamesAreTrimmedAndComparedWithoutCase(t *testing.T) {
	cs := []*rawCommit{commitBy(" Voronkov  Vitalii", "v@one.com"), commitBy("voronkov vitalii", "v@two.com"), commitBy("Voronkov Vitalii", "v@one.com")}
	canonicalizeAuthors(cs)
	for _, c := range cs {
		assert.Equal(t, "Voronkov Vitalii", c.AuthorName)
	}
}

func TestAFullNameIsPreferredToAMoreUsedHandle(t *testing.T) {
	cs := []*rawCommit{commitBy("Marie Standeven", "marie@gmail.com")}
	for i := 0; i < 3; i++ {
		cs = append(cs, commitBy("marieStandeven", "34657994+marieStandeven@users.noreply.github.com"))
	}
	canonicalizeAuthors(cs)
	for _, c := range cs {
		assert.Equal(t, "Marie Standeven", c.AuthorName)
	}
}
