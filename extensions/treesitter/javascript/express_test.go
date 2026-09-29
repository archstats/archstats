package javascript

import (
	"testing"

	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An Express app the way most of them are still written: CommonJS, with
// require() and module.exports and no `import` anywhere.
var expressCJS = map[string]string{
	"api/server/index.js": `
const express = require('express');
const usersRouter = require('./routes/users');
const { errorHandler } = require('./middleware/error');
const app = express();
app.use('/api/users', usersRouter);
app.use(errorHandler);
module.exports = app;
`,
	"api/server/routes/users.js": `
const express = require('express');
const { requireAuth } = require('../middleware/auth');
const { listUsers, createUser } = require('../controllers/users');

const router = express.Router();
router.get('/', requireAuth, listUsers);
router.post('/', requireAuth, createUser);
router.delete('/:id', async (req, res) => { res.sendStatus(204); });

module.exports = router;
`,
	"api/server/controllers/users.js": `
const { findAllUsers, createOne } = require('../services/users');

async function listUsers(req, res) { res.json(await findAllUsers()); }
const createUser = async (req, res) => { res.status(201).json(await createOne(req.body)); };

module.exports = { listUsers, createUser };
`,
	"api/server/middleware/auth.js": `
function requireAuth(req, res, next) {
  if (!req.headers.authorization) return res.sendStatus(401);
  next();
}
module.exports = { requireAuth };
`,
	"api/server/middleware/error.js": `
const errorHandler = (err, req, res, next) => res.status(500).json({ error: err.message });
module.exports = { errorHandler };
`,
	"api/server/services/users.js": `
const User = require('../models/User');

async function findAllUsers() { return User.find().lean(); }
async function createOne(body) { return User.create(body); }

module.exports = { findAllUsers, createOne };
`,
	// A Mongoose model: a schema and a model, both values.
	"api/server/models/User.js": `
const mongoose = require('mongoose');
const userSchema = new mongoose.Schema({ name: String });
module.exports = mongoose.model('User', userSchema);
`,
}

func TestACommonJSExpressAppReadsThroughRequire(t *testing.T) {
	var all []*unit.Unit
	pack := createJavaScriptLanguagePack()
	for p, src := range expressCJS {
		res := pack.AnalyzeFileContent(p, []byte(src))
		require.NotNil(t, res, p)
		all = append(all, common.JSUnitsFrom(p, []byte(src), res)...)
	}
	byID := map[string]*unit.Unit{}
	for _, u := range all {
		byID[u.ID] = u
	}
	edges := map[string]bool{}
	for _, c := range unit.Connections(all) {
		edges[c.From+" -> "+c.To] = true
	}

	for _, id := range []string{
		"api/server/controllers/users#listUsers", "api/server/controllers/users#createUser",
		"api/server/middleware/auth#requireAuth", "api/server/middleware/error#errorHandler",
		"api/server/services/users#findAllUsers", "api/server/services/users#createOne",
	} {
		u := byID[id]
		require.NotNil(t, u, id)
		assert.Equal(t, unit.KindFunction, u.Kind, id)
	}
	// A destructured require is an import binding like any other.
	assert.True(t, edges["api/server/controllers/users#listUsers -> api/server/services/users#findAllUsers"], "%v", edges)
	assert.True(t, edges["api/server/controllers/users#createUser -> api/server/services/users#createOne"])
	assert.True(t, edges["api/server/routes/users# -> api/server/controllers/users#listUsers"])
	assert.True(t, edges["api/server/routes/users# -> api/server/middleware/auth#requireAuth"])

	// Nothing in the models file declares a name: the schema and the model
	// are values, and `module.exports = mongoose.model(...)` names nothing.
	// The service's `require('../models/User')` therefore reaches no unit,
	// and the data layer of a Mongoose app is empty. Read as a gap.
	for id := range byID {
		assert.NotContains(t, id, "models/User#User")
	}
	assert.False(t, edges["api/server/services/users#findAllUsers -> api/server/models/User#User"])
	assert.Contains(t, byID["api/server/services/users#findAllUsers"].Refs, unit.Ref{Module: "../models/User", Name: "User"})
	// The raw requires are what detection reads.
	assert.Contains(t, byID["api/server/routes/users#"].Refs, unit.Ref{Module: "express", Name: "express"})
}
