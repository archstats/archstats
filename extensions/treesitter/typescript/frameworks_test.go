package typescript

import (
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/archstats/archstats/extensions/treesitter/javascript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mini-apps the framework lanes are checked against, one per framework,
// read the way a scan reads them. The UI's layers.web.test.ts builds its
// facts from what these tests assert, so a change here is a change there.

type app struct {
	units map[string]*unit.Unit
	edges map[string]bool
}

func (a app) unit(t *testing.T, id string) *unit.Unit {
	t.Helper()
	u := a.units[id]
	require.NotNil(t, u, "no unit %s; have %v", id, sortedKeys(a.units))
	return u
}

func (a app) hasEdge(t *testing.T, from, to string) {
	t.Helper()
	assert.True(t, a.edges[from+" -> "+to], "missing edge %s -> %s; have %v", from, to, sortedKeys(a.edges))
}

func (a app) noEdge(t *testing.T, from, to string) {
	t.Helper()
	assert.False(t, a.edges[from+" -> "+to], "unexpected edge %s -> %s", from, to)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// analyzeApp feeds each file through the pack a scan would: .vue and
// .svelte through the component analyzer, .tsx through the TSX grammar, .js
// and .jsx through JavaScript, the rest through TypeScript.
func analyzeApp(t *testing.T, files map[string]string) app {
	t.Helper()
	sfc := newSFCAnalyzer()
	ts := createTypeScriptLanguagePack(false)
	tsx := createTypeScriptLanguagePack(true)
	js := javascript.LanguagePack()
	var all []*unit.Unit
	for _, p := range sortedKeys(files) {
		src := []byte(files[p])
		var units []*unit.Unit
		switch strings.ToLower(path.Ext(p)) {
		case ".vue", ".svelte":
			res := sfc.analyze(p, src)
			require.NotNil(t, res, p)
			units = res.Units
		default:
			pack := ts
			switch strings.ToLower(path.Ext(p)) {
			case ".tsx":
				pack = tsx
			case ".js", ".jsx", ".mjs", ".cjs":
				pack = js
			}
			res := pack.AnalyzeFileContent(p, src)
			require.NotNil(t, res, p)
			common.KeepReactOnlyWhereUsed(p, res)
			units = common.JSUnitsFrom(p, src, res)
		}
		all = append(all, units...)
	}
	all = unit.Merge(all)
	out := app{units: unitsByID(all), edges: map[string]bool{}}
	for _, c := range unit.Connections(all) {
		out.edges[c.From+" -> "+c.To] = true
	}
	return out
}

func markerKeys(u *unit.Unit, source string) []string {
	var out []string
	for _, m := range u.Markers {
		if m.Source == source {
			out = append(out, m.Key)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// NestJS: module, controller, service, repository, entity and DTO, a guard
// and a pipe. Everything but the controller, the module and the DTO is
// `@Injectable()`, which is why that decorator alone can place nothing.
// ---------------------------------------------------------------------------

var nestApp = map[string]string{
	"src/users/users.module.ts": `
import { Module } from '@nestjs/common'
import { TypeOrmModule } from '@nestjs/typeorm'
import { UsersController } from './users.controller'
import { UsersService } from './users.service'
import { UsersRepository } from './users.repository'
import { User } from './user.entity'

@Module({
  imports: [TypeOrmModule.forFeature([User])],
  controllers: [UsersController],
  providers: [UsersService, UsersRepository],
})
export class UsersModule {}
`,
	"src/users/users.controller.ts": `
import { Body, Controller, Get, Post, UseGuards, UsePipes } from '@nestjs/common'
import { UsersService } from './users.service'
import { CreateUserDto } from './dto/create-user.dto'
import { AuthGuard } from '../auth/auth.guard'
import { ValidationPipe } from '../common/validation.pipe'

@Controller('users')
@UseGuards(AuthGuard)
export class UsersController {
  constructor(private readonly users: UsersService) {}

  @Get()
  findAll() { return this.users.findAll() }

  @Post()
  @UsePipes(ValidationPipe)
  create(@Body() dto: CreateUserDto) { return this.users.create(dto) }
}
`,
	// A controller that injects the repository itself, skipping the service.
	"src/audit/audit.controller.ts": `
import { Controller, Get } from '@nestjs/common'
import { UsersRepository } from '../users/users.repository'

@Controller('audit')
export class AuditController {
  constructor(private readonly users: UsersRepository) {}
  @Get() all() { return this.users.find() }
}
`,
	"src/users/users.service.ts": `
import { Injectable } from '@nestjs/common'
import { UsersRepository } from './users.repository'
import { CreateUserDto } from './dto/create-user.dto'
import { User } from './user.entity'

@Injectable()
export class UsersService {
  constructor(private readonly repo: UsersRepository) {}
  findAll(): Promise<User[]> { return this.repo.find() }
  create(dto: CreateUserDto) { return this.repo.save(dto) }
}
`,
	"src/users/users.repository.ts": `
import { Injectable } from '@nestjs/common'
import { InjectRepository } from '@nestjs/typeorm'
import { Repository } from 'typeorm'
import { User } from './user.entity'

@Injectable()
export class UsersRepository {
  constructor(@InjectRepository(User) private readonly users: Repository<User>) {}
  find() { return this.users.find() }
  save(u: Partial<User>) { return this.users.save(u) }
}
`,
	// The planted violation: an entity reaching up into a service.
	"src/users/user.entity.ts": `
import { Column, Entity, PrimaryGeneratedColumn } from 'typeorm'
import { UsersService } from './users.service'

@Entity()
export class User {
  @PrimaryGeneratedColumn() id: number
  @Column() name: string
  static async count(users: UsersService) { return (await users.findAll()).length }
}
`,
	"src/users/dto/create-user.dto.ts": `
import { IsEmail, IsString } from 'class-validator'

export class CreateUserDto {
  @IsString() name: string
  @IsEmail() email: string
}

export class UpdateUserDto {
  @IsString() name?: string
}
`,
	"src/auth/auth.guard.ts": `
import { CanActivate, ExecutionContext, Injectable } from '@nestjs/common'

@Injectable()
export class AuthGuard implements CanActivate {
  canActivate(ctx: ExecutionContext) { return !!ctx.switchToHttp().getRequest().user }
}
`,
	"src/common/validation.pipe.ts": `
import { Injectable, PipeTransform } from '@nestjs/common'

@Injectable()
export class ValidationPipe implements PipeTransform {
  transform(value: unknown) { return value }
}
`,
}

func TestNestJSUnitsCarryTheDecoratorsAndHeritageTheLanesRead(t *testing.T) {
	a := analyzeApp(t, nestApp)

	for id, want := range map[string][]string{
		"src/users/users.module#UsersModule":         {"Module"},
		"src/users/users.controller#UsersController": {"Controller", "UseGuards"},
		"src/audit/audit.controller#AuditController": {"Controller"},
		"src/users/users.service#UsersService":       {"Injectable"},
		"src/users/users.repository#UsersRepository": {"Injectable"},
		"src/users/user.entity#User":                 {"Entity"},
		"src/auth/auth.guard#AuthGuard":              {"Injectable"},
		"src/common/validation.pipe#ValidationPipe":  {"Injectable"},
	} {
		u := a.unit(t, id)
		assert.Equal(t, unit.KindType, u.Kind, id)
		assert.ElementsMatch(t, want, markerKeys(u, unit.SourceAnnotation), id)
	}
	// What a class implements is what says it is a guard or a pipe, since
	// both are @Injectable() like every service.
	assert.Equal(t, []string{"CanActivate"}, markerKeys(a.unit(t, "src/auth/auth.guard#AuthGuard"), unit.SourceSupertype))
	assert.Equal(t, []string{"PipeTransform"}, markerKeys(a.unit(t, "src/common/validation.pipe#ValidationPipe"), unit.SourceSupertype))
}

// `@IsString()` on a field, `@Get()` on a handler, `@Column()` on a column
// and `@InjectRepository()` on a parameter are about members. They used to
// fall on the next class declared in the file, so UpdateUserDto was marked
// IsString and IsEmail by CreateUserDto's fields.
func TestMemberDecoratorsAreNotTheNextClasss(t *testing.T) {
	a := analyzeApp(t, nestApp)
	assert.Empty(t, a.unit(t, "src/users/dto/create-user.dto#CreateUserDto").Markers)
	assert.Empty(t, a.unit(t, "src/users/dto/create-user.dto#UpdateUserDto").Markers)
	assert.Equal(t, []string{"Entity"}, markerKeys(a.unit(t, "src/users/user.entity#User"), unit.SourceAnnotation))
	// Only the DTO's name says what it is.
	assert.Equal(t, "CreateUserDto", a.unit(t, "src/users/dto/create-user.dto#CreateUserDto").Name)
}

func TestNestJSReferencesRunFromControllerToServiceToRepositoryToEntity(t *testing.T) {
	a := analyzeApp(t, nestApp)
	// The module wires everything: its decorator's arguments are its own uses.
	for _, to := range []string{
		"src/users/users.controller#UsersController",
		"src/users/users.service#UsersService",
		"src/users/users.repository#UsersRepository",
		"src/users/user.entity#User",
	} {
		a.hasEdge(t, "src/users/users.module#UsersModule", to)
	}
	// The controller: its guard, the service its constructor takes, the DTO
	// and the pipe its handler names.
	a.hasEdge(t, "src/users/users.controller#UsersController", "src/auth/auth.guard#AuthGuard")
	a.hasEdge(t, "src/users/users.controller#UsersController.constructor", "src/users/users.service#UsersService")
	a.hasEdge(t, "src/users/users.controller#UsersController.create", "src/users/dto/create-user.dto#CreateUserDto")
	// A handler's decorator sits in the class body before the handler, so
	// what it names is the class's use rather than the handler's; both roll
	// up to the controller.
	a.hasEdge(t, "src/users/users.controller#UsersController", "src/common/validation.pipe#ValidationPipe")
	// Service to repository, repository to entity.
	a.hasEdge(t, "src/users/users.service#UsersService.constructor", "src/users/users.repository#UsersRepository")
	a.hasEdge(t, "src/users/users.service#UsersService.findAll", "src/users/user.entity#User")
	a.hasEdge(t, "src/users/users.repository#UsersRepository.constructor", "src/users/user.entity#User")
	// The controller that skips the service.
	a.hasEdge(t, "src/audit/audit.controller#AuditController.constructor", "src/users/users.repository#UsersRepository")
	// The planted violation: the entity reaching up to the service.
	a.hasEdge(t, "src/users/user.entity#User.count", "src/users/users.service#UsersService")
	// Framework packages resolve to nothing and make no edge.
	for id := range a.edges {
		assert.NotContains(t, id, "@nestjs", id)
		assert.NotContains(t, id, "typeorm#", id)
	}
}

// ---------------------------------------------------------------------------
// Angular: a standalone component with inject() and a signal, a service, a
// data class over HttpClient, a model, a directive, a pipe and a functional
// guard. The data class and the service are both @Injectable().
// ---------------------------------------------------------------------------

var angularApp = map[string]string{
	"src/app/heroes/hero-list.component.ts": `
import { Component, Input, inject, signal } from '@angular/core'
import { Store } from '@ngrx/store'
import { HeroService } from './hero.service'
import { Hero } from './hero.model'
import { HighlightDirective } from '../shared/highlight.directive'
import { TitleCasePipe } from '../shared/title-case.pipe'

@Component({
  selector: 'app-hero-list',
  standalone: true,
  imports: [HighlightDirective, TitleCasePipe],
  template: '<li *ngFor="let h of list()" appHighlight>{{ h.name | titleCase }}</li>',
})
export class HeroListComponent {
  private readonly heroes = inject(HeroService)
  private readonly store = inject(Store)
  readonly list = signal<Hero[]>([])
  @Input() title = ''

  ngOnInit() { this.heroes.load().subscribe(h => this.list.set(h)) }
}
`,
	"src/app/heroes/hero.service.ts": `
import { Injectable, inject } from '@angular/core'
import { HeroApi } from './hero.api'
import { Hero } from './hero.model'

@Injectable({ providedIn: 'root' })
export class HeroService {
  private readonly api = inject(HeroApi)
  load() { return this.api.fetchAll() }
  first(list: Hero[]) { return list[0] }
}
`,
	// The planted violation: the data class reaching up to the service.
	"src/app/heroes/hero.api.ts": `
import { Injectable, inject } from '@angular/core'
import { HttpClient } from '@angular/common/http'
import { Hero } from './hero.model'
import { HeroService } from './hero.service'

@Injectable({ providedIn: 'root' })
export class HeroApi {
  private readonly http = inject(HttpClient)
  private readonly service = inject(HeroService)
  fetchAll() { return this.http.get<Hero[]>('/api/heroes') }
}
`,
	"src/app/heroes/hero.model.ts": `
export interface Hero { id: number; name: string; power: Power }
export type Power = 'flight' | 'strength'
`,
	"src/app/shared/highlight.directive.ts": `
import { Directive, ElementRef, inject } from '@angular/core'

@Directive({ selector: '[appHighlight]', standalone: true })
export class HighlightDirective {
  private readonly el = inject(ElementRef)
}
`,
	"src/app/shared/title-case.pipe.ts": `
import { Pipe, PipeTransform } from '@angular/core'

@Pipe({ name: 'titleCase', standalone: true })
export class TitleCasePipe implements PipeTransform {
  transform(v: string) { return v[0].toUpperCase() + v.slice(1) }
}
`,
	"src/app/auth/auth.guard.ts": `
import { inject } from '@angular/core'
import { CanActivateFn } from '@angular/router'
import { HeroService } from '../heroes/hero.service'

export const authGuard: CanActivateFn = () => !!inject(HeroService)
`,
	"src/app/auth/auth.interceptor.ts": `
import { Injectable } from '@angular/core'
import { HttpInterceptor, HttpRequest, HttpHandler } from '@angular/common/http'

@Injectable()
export class AuthInterceptor implements HttpInterceptor {
  intercept(req: HttpRequest<unknown>, next: HttpHandler) { return next.handle(req) }
}
`,
}

func TestAngularUnitsCarryTheirDecoratorsShapesAndHeritage(t *testing.T) {
	a := analyzeApp(t, angularApp)
	for id, want := range map[string][]string{
		"src/app/heroes/hero-list.component#HeroListComponent":  {"Component"},
		"src/app/heroes/hero.service#HeroService":               {"Injectable"},
		"src/app/heroes/hero.api#HeroApi":                       {"Injectable"},
		"src/app/shared/highlight.directive#HighlightDirective": {"Directive"},
		"src/app/shared/title-case.pipe#TitleCasePipe":          {"Pipe"},
		"src/app/auth/auth.interceptor#AuthInterceptor":         {"Injectable"},
	} {
		assert.ElementsMatch(t, want, markerKeys(a.unit(t, id), unit.SourceAnnotation), id)
	}
	// `@Input()` is about a property, not the class.
	assert.NotContains(t, markerKeys(a.unit(t, "src/app/heroes/hero-list.component#HeroListComponent"), unit.SourceAnnotation), "Input")
	assert.Equal(t, []string{"HttpInterceptor"}, markerKeys(a.unit(t, "src/app/auth/auth.interceptor#AuthInterceptor"), unit.SourceSupertype))
	assert.Equal(t, []string{"PipeTransform"}, markerKeys(a.unit(t, "src/app/shared/title-case.pipe#TitleCasePipe"), unit.SourceSupertype))

	// A functional guard is a function whose type says what it is; the type
	// is a marker nowhere, so its name is all a lane can read.
	guard := a.unit(t, "src/app/auth/auth.guard#authGuard")
	assert.Equal(t, unit.KindFunction, guard.Kind)
	assert.Empty(t, guard.Markers)

	// An interface and a type alias are both shapes.
	assert.Equal(t, []string{"interface"}, markerKeys(a.unit(t, "src/app/heroes/hero.model#Hero"), unit.SourceSupertype))
	assert.Equal(t, []string{"type_alias"}, markerKeys(a.unit(t, "src/app/heroes/hero.model#Power"), unit.SourceSupertype))
	assert.Equal(t, unit.KindType, a.unit(t, "src/app/heroes/hero.model#Power").Kind)
}

func TestAngularReferencesRunFromComponentToServiceToApiToModel(t *testing.T) {
	a := analyzeApp(t, angularApp)
	// inject() in a field initialiser is the class's own use.
	c := "src/app/heroes/hero-list.component#HeroListComponent"
	a.hasEdge(t, c, "src/app/heroes/hero.service#HeroService")
	a.hasEdge(t, c, "src/app/heroes/hero.model#Hero")
	a.hasEdge(t, c, "src/app/shared/highlight.directive#HighlightDirective")
	a.hasEdge(t, c, "src/app/shared/title-case.pipe#TitleCasePipe")
	a.hasEdge(t, "src/app/heroes/hero.service#HeroService", "src/app/heroes/hero.api#HeroApi")
	a.hasEdge(t, "src/app/heroes/hero.service#HeroService.first", "src/app/heroes/hero.model#Hero")
	a.hasEdge(t, "src/app/heroes/hero.api#HeroApi.fetchAll", "src/app/heroes/hero.model#Hero")
	a.hasEdge(t, "src/app/auth/auth.guard#authGuard", "src/app/heroes/hero.service#HeroService")
	// The planted violation.
	a.hasEdge(t, "src/app/heroes/hero.api#HeroApi", "src/app/heroes/hero.service#HeroService")
	// A name used from the same file is not imported, so it is no reference.
	a.noEdge(t, "src/app/heroes/hero.model#Hero", "src/app/heroes/hero.model#Power")
}

// ---------------------------------------------------------------------------
// React with Next.js: a route, components, a hook, an API client and types.
// ---------------------------------------------------------------------------

var reactApp = map[string]string{
	"app/users/page.tsx": `
import { UserList } from '@/components/UserList'
import { useUsers } from '@/hooks/useUsers'

export default function Page() {
  const { data } = useUsers()
  return <UserList users={data ?? []} />
}
`,
	"pages/index.tsx": `
import Link from 'next/link'
import { UserCard } from '@/components/UserCard'

export default function Home() { return <Link href="/users"><UserCard user={{ id: '1', name: 'Ada', role: 'admin' }} /></Link> }
`,
	"components/UserList.tsx": `
import { memo } from 'react'
import axios from 'axios'
import type { User } from '@/types/user'
import { UserCard } from './UserCard'

export const UserList = memo(function UserList({ users }: { users: User[] }) {
  return <ul>{users.map(u => <UserCard key={u.id} user={u} />)}</ul>
})

// A component that talks to the server itself is still a component.
export function RefreshButton() {
  const onClick = async () => { await axios.post('/api/refresh') }
  return <button onClick={onClick}>Refresh</button>
}
`,
	"components/UserCard.tsx": `
import { forwardRef } from 'react'
import type { User } from '@/types/user'

export const UserCard = forwardRef<HTMLLIElement, { user: User }>(({ user }, ref) => <li ref={ref}>{user.name}</li>)
`,
	// The planted violation: a hook that reaches up into a component.
	"hooks/useUsers.ts": `
import { useQuery } from '@tanstack/react-query'
import { fetchUsers } from '@/lib/api/users'
import type { User } from '@/types/user'
import { UserCard } from '@/components/UserCard'

export function useUsers() {
  return useQuery<User[]>({ queryKey: ['users'], queryFn: fetchUsers, meta: { render: UserCard } })
}
`,
	"lib/api/users.ts": `
import axios from 'axios'
import type { User } from '@/types/user'

export async function fetchUsers(): Promise<User[]> {
  const r = await axios.get<User[]>('/api/users')
  return r.data
}
`,
	"types/user.ts": `
export interface User { id: string; name: string; role: Role }
export type Role = 'admin' | 'member'
`,
}

func TestReactUnitsAreFunctionsHooksAndShapes(t *testing.T) {
	a := analyzeApp(t, reactApp)
	for _, id := range []string{
		"app/users/page#Page", "pages#Home", "components/UserList#UserList", "components/UserList#RefreshButton",
		"components/UserCard#UserCard", "hooks/useUsers#useUsers", "lib/api/users#fetchUsers",
	} {
		u := a.unit(t, id)
		assert.Equal(t, unit.KindFunction, u.Kind, id)
		assert.Empty(t, u.Owner, id)
		assert.Empty(t, markerKeys(u, unit.SourceAnnotation), id)
	}
	// A component wrapped in memo() or forwardRef() is that component, and
	// carries the wrapper as what made it.
	assert.Equal(t, []string{"memo"}, markerKeys(a.unit(t, "components/UserList#UserList"), unit.SourceSupertype))
	assert.Equal(t, []string{"forwardRef"}, markerKeys(a.unit(t, "components/UserCard#UserCard"), unit.SourceSupertype))
	assert.Empty(t, a.unit(t, "app/users/page#Page").Markers)
	// The callback inside RefreshButton is the button's, not a unit of the
	// file that happens to be the one importing axios.
	onClick := a.unit(t, "components/UserList#RefreshButton.onClick")
	assert.Equal(t, "components/UserList#RefreshButton", onClick.Owner)
	assert.Contains(t, onClick.Refs, unit.Ref{Module: "axios", Name: "axios"})
	assert.Nil(t, a.units["components/UserList#onClick"])

	assert.Equal(t, []string{"interface"}, markerKeys(a.unit(t, "types/user#User"), unit.SourceSupertype))
	assert.Equal(t, []string{"type_alias"}, markerKeys(a.unit(t, "types/user#Role"), unit.SourceSupertype))
}

func TestReactReferencesRunFromPageToComponentToHookToClientToTypes(t *testing.T) {
	a := analyzeApp(t, reactApp)
	a.hasEdge(t, "app/users/page#Page", "components/UserList#UserList")
	a.hasEdge(t, "app/users/page#Page", "hooks/useUsers#useUsers")
	// An index file is named by its directory, as an import writes it.
	a.hasEdge(t, "pages#Home", "components/UserCard#UserCard")
	a.hasEdge(t, "components/UserList#UserList", "components/UserCard#UserCard")
	a.hasEdge(t, "components/UserList#UserList", "types/user#User")
	a.hasEdge(t, "components/UserCard#UserCard", "types/user#User")
	a.hasEdge(t, "hooks/useUsers#useUsers", "lib/api/users#fetchUsers")
	a.hasEdge(t, "hooks/useUsers#useUsers", "types/user#User")
	a.hasEdge(t, "lib/api/users#fetchUsers", "types/user#User")
	// The planted violation.
	a.hasEdge(t, "hooks/useUsers#useUsers", "components/UserCard#UserCard")
	// A name used from the same file is not imported, so it is no reference:
	// User's field of type Role makes no edge.
	a.noEdge(t, "types/user#User", "types/user#Role")
	// `next/link`, `react` and `axios` resolve to nothing.
	for id := range a.edges {
		assert.NotContains(t, id, "-> next", id)
		assert.NotContains(t, id, "-> react", id)
		assert.NotContains(t, id, "-> axios", id)
	}
}

// The shapes a component takes, and the ones it does not.
func TestReactComponentShapes(t *testing.T) {
	a := analyzeApp(t, map[string]string{
		"src/Shapes.tsx": `
import React, { forwardRef, memo, useState } from 'react'

export function Declared() { return <div /> }
export const Arrow = () => <div />
export class Legacy extends React.Component<{}> {
  handle = () => {}
  render() { return null }
}
export class Pure extends React.PureComponent {}
export const Wrapped = React.memo(() => <div />)
export const Referenced = forwardRef<HTMLDivElement>((_, ref) => <div ref={ref} />)
export const Named = memo(function Named() { return <div /> })
// A higher-order component and what it returns.
const withAuth = (C: React.ComponentType) => (p: {}) => <C {...p} />
export const Guarded = withAuth(Declared)
// Not a hook: no capital after use.
export const useless = () => 1
export default function () { return <Arrow /> }
`,
		"src/legacy/Button.jsx": `
import React from 'react'
export default function Button({ label }) { return <button>{label}</button> }
export const Icon = () => <i />
`,
		"src/legacy/Panel.js": `
import React from 'react'
import { Icon } from './Button.jsx'
export function Panel() { return React.createElement('div', null, React.createElement(Icon)) }
`,
	})
	for _, name := range []string{"Declared", "Arrow", "Wrapped", "Referenced", "Named", "withAuth", "useless"} {
		u := a.unit(t, "src/Shapes#"+name)
		assert.Equal(t, unit.KindFunction, u.Kind, name)
		assert.Empty(t, u.Owner, name)
	}
	// Class components carry what they extend.
	assert.Equal(t, []string{"Component"}, markerKeys(a.unit(t, "src/Shapes#Legacy"), unit.SourceSupertype))
	assert.Equal(t, []string{"PureComponent"}, markerKeys(a.unit(t, "src/Shapes#Pure"), unit.SourceSupertype))
	// A class field holding an arrow function is a field, not a unit.
	assert.Nil(t, a.units["src/Shapes#Legacy.handle"])
	assert.Nil(t, a.units["src/Shapes#handle"])
	// A component made by calling a HOC is no declaration of its own: the
	// wrapped component is the unit. And an anonymous default export has no
	// name to be a unit by.
	assert.Nil(t, a.units["src/Shapes#Guarded"])
	assert.Nil(t, a.units["src/Shapes#Shapes"])
	// .jsx and .js through the JavaScript grammar.
	assert.Equal(t, unit.KindFunction, a.unit(t, "src/legacy/Button#Button").Kind)
	assert.Equal(t, unit.KindFunction, a.unit(t, "src/legacy/Button#Icon").Kind)
	assert.Equal(t, unit.KindFunction, a.unit(t, "src/legacy/Panel#Panel").Kind)
	a.hasEdge(t, "src/legacy/Panel#Panel", "src/legacy/Button#Icon")
}

// ---------------------------------------------------------------------------
// Vue with Nuxt: the root, a layout, a page, components in both APIs, a
// composable, a setup store and an options store, an API client and types.
// ---------------------------------------------------------------------------

var vueApp = map[string]string{
	// The root and the layout use Nuxt's auto-imported components: no
	// import line, so nothing to make an edge from. Root files are written
	// with the walker's "./" prefix.
	"./app.vue": `<template><NuxtLayout><NuxtPage /></NuxtLayout></template>
`,
	"layouts/default.vue": `<template>
  <AppHeader />
  <slot />
</template>
`,
	"./error.vue": `<template><h1>{{ error.statusCode }}</h1></template>
<script setup lang="ts">
defineProps<{ error: { statusCode: number } }>()
</script>
`,
	"pages/cart/[id].vue": `<template>
  <CartPanel :items="items" />
  <AppHeader />
</template>

<script setup lang="ts">
import CartPanel from '~/components/CartPanel.vue'
import { useCart } from '~/composables/useCart'

const { items } = useCart()
</script>
`,
	"components/AppHeader.vue": `<template><header /></template>
`,
	"components/CartPanel.vue": `<template>
  <ul><cart-line v-for="i in items" :key="i.id" :item="i" /></ul>
</template>

<script setup lang="ts">
import CartLine from './CartLine.vue'
import type { CartItem } from '~/types/cart'

interface Props { items: CartItem[] }
const props = defineProps<Props>()
const emit = defineEmits<{ select: [item: CartItem] }>()
function pick(i: CartItem) { emit('select', i) }
</script>
`,
	// The Options API, in plain JavaScript.
	"components/CartLine.vue": `<script>
import { formatPrice } from '~/utils/price'

export default {
  props: ['item'],
  methods: {
    price() { return formatPrice(this.item.price) },
  },
}
</script>

<template><li>{{ price() }}</li></template>
`,
	"composables/useCart.ts": `
import { computed } from 'vue'
import { useCartStore } from '~/stores/cart'

export function useCart() {
  const store = useCartStore()
  const items = computed(() => store.items)
  return { items, load: store.load }
}
`,
	"stores/cart.ts": `
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchCart } from '~/api/cart'
import type { CartItem } from '~/types/cart'

export const useCartStore = defineStore('cart', () => {
  const items = ref<CartItem[]>([])
  async function load() { items.value = await fetchCart() }
  return { items, load }
})
`,
	"stores/user.ts": `
import { defineStore } from 'pinia'

export const useUserStore = defineStore('user', {
  state: () => ({ name: '' }),
  actions: {
    rename(n: string) { this.name = n },
  },
})
`,
	// The planted violation: the API client reading the store.
	"api/cart.ts": `
import { ofetch } from 'ofetch'
import type { CartItem } from '~/types/cart'
import { useUserStore } from '~/stores/user'

export function fetchCart() {
  return ofetch<CartItem[]>('/api/cart', { headers: { 'x-user': useUserStore().name } })
}
`,
	"types/cart.ts": `
export interface CartItem { id: string; price: number }
`,
	"utils/price.ts": `
export function formatPrice(n: number) { return n.toFixed(2) }
`,
}

func TestVueUnitsAreComponentsStoresComposablesAndShapes(t *testing.T) {
	a := analyzeApp(t, vueApp)
	for _, id := range []string{
		"app#app", "layouts/default#default", "error#error", "pages/cart/[id]#[id]",
		"components/AppHeader#AppHeader", "components/CartPanel#CartPanel", "components/CartLine#CartLine",
	} {
		u := a.unit(t, id)
		assert.Equal(t, unit.KindType, u.Kind, id)
		assert.Equal(t, []unit.Marker{{Source: unit.SourceFilename, Key: "vue_component"}}, u.Markers, id)
		assert.Empty(t, u.Owner, id)
	}
	// Both stores are units made by defineStore, whichever API.
	for _, id := range []string{"stores/cart#useCartStore", "stores/user#useUserStore"} {
		u := a.unit(t, id)
		assert.Equal(t, unit.KindType, u.Kind, id)
		assert.Equal(t, []string{"defineStore"}, markerKeys(u, unit.SourceSupertype), id)
	}
	assert.Equal(t, "stores/cart#useCartStore", a.unit(t, "stores/cart#useCartStore.load").Owner)
	assert.Equal(t, "stores/user#useUserStore", a.unit(t, "stores/user#useUserStore.rename").Owner)

	// A composable and a client are plain functions; only the name tells
	// one from the other.
	assert.Equal(t, unit.KindFunction, a.unit(t, "composables/useCart#useCart").Kind)
	assert.Equal(t, unit.KindFunction, a.unit(t, "api/cart#fetchCart").Kind)
	assert.Equal(t, []string{"interface"}, markerKeys(a.unit(t, "types/cart#CartItem"), unit.SourceSupertype))

	// What a <script setup> declares is the component's: its function, and
	// its props interface, which is no model of the codebase.
	assert.Equal(t, "components/CartPanel#CartPanel", a.unit(t, "components/CartPanel#pick").Owner)
	assert.Equal(t, "components/CartPanel#CartPanel", a.unit(t, "components/CartPanel#Props").Owner)
	assert.Equal(t, "components/CartLine#CartLine", a.unit(t, "components/CartLine#price").Owner)
	// The compiler macros are values.
	assert.Nil(t, a.units["components/CartPanel#props"])
	assert.Nil(t, a.units["components/CartPanel#emit"])
}

func TestVueReferencesRunFromPageToComponentToComposableToStoreToClientToTypes(t *testing.T) {
	a := analyzeApp(t, vueApp)
	a.hasEdge(t, "pages/cart/[id]#[id]", "components/CartPanel#CartPanel")
	a.hasEdge(t, "pages/cart/[id]#[id]", "composables/useCart#useCart")
	a.hasEdge(t, "components/CartPanel#CartPanel", "components/CartLine#CartLine")
	a.hasEdge(t, "components/CartPanel#CartPanel", "types/cart#CartItem")
	a.hasEdge(t, "components/CartLine#price", "utils/price#formatPrice")
	a.hasEdge(t, "composables/useCart#useCart", "stores/cart#useCartStore")
	a.hasEdge(t, "stores/cart#useCartStore.load", "api/cart#fetchCart")
	a.hasEdge(t, "stores/cart#useCartStore", "types/cart#CartItem")
	a.hasEdge(t, "api/cart#fetchCart", "types/cart#CartItem")
	// The planted violation.
	a.hasEdge(t, "api/cart#fetchCart", "stores/user#useUserStore")
}

// Nuxt registers everything under components/ without an import line, so a
// template that uses <AppHeader /> makes no edge to AppHeader: there is no
// import to read the module from. The layout and the page both use it and
// neither is connected to it. Read as a gap, not fixed here: closing it
// means matching template tags to component files by name across the
// codebase, which is the resolver's job rather than a file's.
func TestNuxtAutoImportedComponentsMakeNoEdge(t *testing.T) {
	a := analyzeApp(t, vueApp)
	a.noEdge(t, "layouts/default#default", "components/AppHeader#AppHeader")
	a.noEdge(t, "pages/cart/[id]#[id]", "components/AppHeader#AppHeader")
	assert.Empty(t, a.unit(t, "layouts/default#default").Refs)
	assert.Empty(t, a.unit(t, "app#app").Refs)
}

// A component in a .ts file, made by defineComponent, is a unit that says so.
func TestADefineComponentConstantIsAComponent(t *testing.T) {
	a := analyzeApp(t, map[string]string{
		"src/components/Badge.ts": `
import { defineComponent, h } from 'vue'
export const Badge = defineComponent({
  props: { label: String },
  setup(props) { return () => h('span', props.label) },
})
`,
	})
	badge := a.unit(t, "src/components/Badge#Badge")
	assert.Equal(t, unit.KindType, badge.Kind)
	assert.Equal(t, []string{"defineComponent"}, markerKeys(badge, unit.SourceSupertype))
	assert.Equal(t, badge.ID, a.unit(t, "src/components/Badge#Badge.setup").Owner)
}

// ---------------------------------------------------------------------------
// Express in TypeScript: app, router, middleware, handlers, service,
// repository, model.
// ---------------------------------------------------------------------------

var expressApp = map[string]string{
	"src/app.ts": `
import express from 'express'
import { usersRouter } from './routes/users'
import { errorHandler } from './middleware/error'

const app = express()
app.use('/users', usersRouter)
app.use(errorHandler)
export default app
`,
	"src/routes/users.ts": `
import { Router } from 'express'
import { requireAuth } from '../middleware/auth'
import { createUser, listUsers } from '../controllers/users.controller'

export const usersRouter = Router()
usersRouter.get('/', requireAuth, listUsers)
usersRouter.post('/', requireAuth, createUser)
usersRouter.delete('/:id', async (req, res) => { res.sendStatus(204) })
`,
	"src/controllers/users.controller.ts": `
import { Request, Response } from 'express'
import { createOne, findAllUsers } from '../services/users.service'

export async function listUsers(req: Request, res: Response) { res.json(await findAllUsers()) }
export const createUser = async (req: Request, res: Response) => { res.status(201).json(await createOne(req.body)) }
`,
	"src/middleware/auth.ts": `
import { NextFunction, Request, Response } from 'express'

export function requireAuth(req: Request, res: Response, next: NextFunction) {
  if (!req.headers.authorization) return res.sendStatus(401)
  next()
}
`,
	"src/middleware/error.ts": `
import { NextFunction, Request, Response } from 'express'
export const errorHandler = (err: Error, req: Request, res: Response, next: NextFunction) => res.status(500).json({ error: err.message })
`,
	"src/services/users.service.ts": `
import { UserRepository } from '../repositories/user.repository'
import { User } from '../models/user'

const repo = new UserRepository()
export function findAllUsers(): Promise<User[]> { return repo.findAll() }
export function createOne(u: User) { return repo.insert(u) }
`,
	// The planted violation: the repository reaching up to a handler.
	"src/repositories/user.repository.ts": `
import { Pool } from 'pg'
import { User } from '../models/user'
import { listUsers } from '../controllers/users.controller'

export class UserRepository {
  private readonly pool = new Pool()
  async findAll(): Promise<User[]> { return (await this.pool.query('select * from users')).rows }
  insert(u: User) { return this.pool.query('insert into users values ($1)', [u.name]) }
  audit() { return listUsers }
}
`,
	"src/models/user.ts": `
export interface User { id: string; name: string }
`,
}

func TestExpressUnitsAreHandlersMiddlewareServicesAndRepositories(t *testing.T) {
	a := analyzeApp(t, expressApp)
	for _, id := range []string{
		"src/controllers/users.controller#listUsers", "src/controllers/users.controller#createUser",
		"src/middleware/auth#requireAuth", "src/middleware/error#errorHandler",
		"src/services/users.service#findAllUsers", "src/services/users.service#createOne",
	} {
		u := a.unit(t, id)
		assert.Equal(t, unit.KindFunction, u.Kind, id)
		assert.Empty(t, u.Markers, id)
	}
	assert.Equal(t, unit.KindType, a.unit(t, "src/repositories/user.repository#UserRepository").Kind)
	assert.Equal(t, []string{"interface"}, markerKeys(a.unit(t, "src/models/user#User"), unit.SourceSupertype))

	// A handler's use of express's types is what says it answers a request.
	assert.Contains(t, a.unit(t, "src/controllers/users.controller#listUsers").Refs, unit.Ref{Module: "express", Name: "Request"})
	assert.Contains(t, a.unit(t, "src/middleware/auth#requireAuth").Refs, unit.Ref{Module: "express", Name: "NextFunction"})
}

// `Router()` makes a value, not a declaration, and a handler written inline
// as `router.get('/', async (req, res) => …)` is anonymous. Neither is a
// unit, so the router file is its module unit and nothing else: its uses
// are the module's, and an anonymous handler's are too.
func TestAnExpressRouterAndItsInlineHandlersAreNotUnits(t *testing.T) {
	a := analyzeApp(t, expressApp)
	assert.Nil(t, a.units["src/routes/users#usersRouter"])
	assert.Nil(t, a.units["src/routes/users#router"])
	mod := a.unit(t, "src/routes/users#")
	assert.Equal(t, unit.KindModule, mod.Kind)
	a.hasEdge(t, "src/routes/users#", "src/controllers/users.controller#listUsers")
	a.hasEdge(t, "src/routes/users#", "src/controllers/users.controller#createUser")
	a.hasEdge(t, "src/routes/users#", "src/middleware/auth#requireAuth")
	a.hasEdge(t, "src/app#", "src/middleware/error#errorHandler")
	// Only a unit with a name can be depended on.
	for id := range a.edges {
		assert.False(t, strings.HasSuffix(id, "#"), id)
	}
}

func TestExpressReferencesRunFromHandlerToServiceToRepositoryToModel(t *testing.T) {
	a := analyzeApp(t, expressApp)
	a.hasEdge(t, "src/controllers/users.controller#listUsers", "src/services/users.service#findAllUsers")
	a.hasEdge(t, "src/controllers/users.controller#createUser", "src/services/users.service#createOne")
	a.hasEdge(t, "src/services/users.service#findAllUsers", "src/models/user#User")
	a.hasEdge(t, "src/repositories/user.repository#UserRepository.findAll", "src/models/user#User")
	// The planted violation.
	a.hasEdge(t, "src/repositories/user.repository#UserRepository.audit", "src/controllers/users.controller#listUsers")
	// `const repo = new UserRepository()` at the top of the service is the
	// module's use, not any function's: a handler calling `repo.findAll()`
	// names a local, so the service-to-repository edge is the module's.
	a.hasEdge(t, "src/services/users.service#", "src/repositories/user.repository#UserRepository")
	a.noEdge(t, "src/services/users.service#findAllUsers", "src/repositories/user.repository#UserRepository")
}
