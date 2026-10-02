//! Browsing: home, the movie/series/genre listings, title detail, genres, My List and search,
//! cached stale-while-revalidate. A surface a shell has open is kept fresh: opening it shows
//! what is cached (marked stale) and refreshes it; returning to the foreground or switching the
//! display language refreshes everything open. The last home is kept per account, so a cold
//! start paints before the network answers.

mod views;

use std::collections::{HashMap, VecDeque};

use couchverse_api::ops::{
    BrowseTitlesQuery, GetHomeQuery, GetTitleQuery, ListGenresQuery, ListWatchlistQuery,
    SearchQuery,
};
use couchverse_api::types::{
    BrowsePage, BrowseTitlesKind, CardItem, Genre, Home, SearchResults, TitleDetail,
};
use couchverse_api::{Call, NoContent, ops};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

pub use views::*;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{EffectOutput, LoadStatus, Problem, Surface, U53};
use crate::modules::images::Images;

/// How long a loaded surface counts as fresh: reopening it within this window does not refetch.
const FRESH_MS: U53 = 60_000;
/// Typing pauses this long before a search is sent.
const SEARCH_DEBOUNCE_MS: U53 = 250;
const MAX_TITLES: usize = 30;
const MAX_LISTINGS: usize = 12;

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SearchText {
    pub query: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WatchlistChange {
    pub title_id: String,
    /// True to add the title to My List, false to remove it.
    pub listed: bool,
}

/// Everything the catalog needs from the active session to make a request.
pub struct Env<'a> {
    pub endpoint: &'a Endpoint,
    pub language: &'a str,
}

/// A request in flight; answers from before a reset (another account, another language) are
/// recognized by their generation and dropped.
#[derive(Debug, Clone, PartialEq)]
pub struct CatalogPending {
    generation: u64,
    request: Request,
}

#[derive(Debug, Clone, PartialEq)]
enum Request {
    Home(Call<Home>),
    Title { slug: String, call: Call<TitleDetail> },
    Browse { key: BrowseKey, page: u32, call: Call<BrowsePage> },
    Genres(Call<Vec<Genre>>),
    MyList(Call<Vec<CardItem>>),
    SearchTimer { seq: u64 },
    Search { seq: u64, call: Call<SearchResults> },
    Watchlist { title_id: String, listed: bool, call: Call<NoContent> },
    WarmHome,
}

/// What a resolved output means for the rest of the core.
#[derive(Debug, PartialEq, Eq)]
pub enum CatalogChange {
    None,
    Unauthorized,
    /// A My List change failed and was rolled back; the shell should hear about it.
    WatchlistFailed,
}

/// One cached payload and where its loading stands.
struct Slot<T> {
    value: Option<T>,
    /// When the value was fetched; `None` for a warm-start or invalidated value.
    fetched_at: Option<U53>,
    loading: bool,
    problem: Option<Problem>,
    not_found: bool,
}

impl<T> Default for Slot<T> {
    fn default() -> Self {
        Self { value: None, fetched_at: None, loading: false, problem: None, not_found: false }
    }
}

impl<T> Slot<T> {
    fn status(&self) -> LoadStatus {
        match (&self.value, self.loading) {
            (Some(_), true) => LoadStatus::Stale,
            (Some(_), false) if self.problem.is_some() => LoadStatus::Stale,
            (Some(_), false) => LoadStatus::Loaded,
            (None, true) => LoadStatus::Loading,
            (None, false) if self.not_found => LoadStatus::NotFound,
            (None, false) if self.problem.is_some() => LoadStatus::Failed,
            (None, false) => LoadStatus::Idle,
        }
    }

    /// Whether a shell opening the surface should load it now.
    fn wants(&self, now: U53, force: bool) -> bool {
        (force || !self.fresh(now)) && !self.loading
    }

    fn fresh(&self, now: U53) -> bool {
        self.value.is_some() && self.fetched_at.is_some_and(|at| now.saturating_sub(at) < FRESH_MS)
    }

    fn loaded(&mut self, value: T, now: U53) {
        self.value = Some(value);
        self.fetched_at = Some(now);
        self.loading = false;
        self.problem = None;
        self.not_found = false;
    }

    fn failed(&mut self, failure: &Failure) {
        self.loading = false;
        self.not_found = matches!(failure, Failure::Api(e) if e.status == 404);
        if self.not_found {
            self.value = None;
        }
        self.problem = Some(failure.problem());
    }

    fn invalidate(&mut self) {
        self.fetched_at = None;
    }
}

#[derive(Default)]
struct Listing {
    slot: Slot<Vec<CardItem>>,
    total: u64,
    /// The next page to request; pages count from 1.
    next_page: u32,
    loading_more: bool,
}

/// A small least-recently-used map, so a long session does not grow without bound.
struct Lru<K, V> {
    entries: HashMap<K, V>,
    order: VecDeque<K>,
    cap: usize,
}

impl<K: std::hash::Hash + Eq + Clone, V: Default> Lru<K, V> {
    fn new(cap: usize) -> Self {
        Self { entries: HashMap::new(), order: VecDeque::new(), cap }
    }

    fn get(&self, key: &K) -> Option<&V> {
        self.entries.get(key)
    }

    fn entry(&mut self, key: &K) -> &mut V {
        if let Some(i) = self.order.iter().position(|k| k == key) {
            self.order.remove(i);
        } else if self.order.len() >= self.cap
            && let Some(oldest) = self.order.pop_front()
        {
            self.entries.remove(&oldest);
        }
        self.order.push_back(key.clone());
        self.entries.entry(key.clone()).or_default()
    }

    fn values_mut(&mut self) -> impl Iterator<Item = &mut V> {
        self.entries.values_mut()
    }

    fn clear(&mut self) {
        self.entries.clear();
        self.order.clear();
    }
}

#[derive(Default)]
struct Search {
    query: String,
    /// Bumped by every keystroke; only the latest search's answer is shown.
    seq: u64,
    timer: Option<U53>,
    slot: Slot<Vec<CardItem>>,
}

pub struct Catalog {
    generation: u64,
    /// Where the last home is kept for a warm start; `None` without an account.
    warm_key: Option<String>,
    home: Slot<Home>,
    titles: Lru<String, Slot<TitleDetail>>,
    listings: Lru<BrowseKey, Listing>,
    genres: Slot<Vec<Genre>>,
    my_list: Slot<Vec<CardItem>>,
    search: Search,
    /// Surfaces a shell has open, with how many times.
    open: HashMap<Surface, usize>,
    listed: Listed,
}

impl Default for Catalog {
    fn default() -> Self {
        Self {
            generation: 0,
            warm_key: None,
            home: Slot::default(),
            titles: Lru::new(MAX_TITLES),
            listings: Lru::new(MAX_LISTINGS),
            genres: Slot::default(),
            my_list: Slot::default(),
            search: Search::default(),
            open: HashMap::new(),
            listed: Listed::new(),
        }
    }
}

/// Stores a load's result in its slot, handing back the failure.
fn fill<T>(slot: &mut Slot<T>, result: Result<T, Failure>, now: U53) -> Option<Failure> {
    match result {
        Ok(value) => {
            slot.loaded(value, now);
            None
        }
        Err(failure) => {
            slot.failed(&failure);
            Some(failure)
        }
    }
}

/// Whether a surface belongs to the catalog.
pub fn owns(surface: &Surface) -> bool {
    matches!(
        surface,
        Surface::Home
            | Surface::Browse(_)
            | Surface::Title(_)
            | Surface::Genres
            | Surface::MyList
            | Surface::Search
    )
}

impl Catalog {
    /// Starts over for a newly active account, reading its warm-start home.
    pub fn activate(&mut self, ctx: &mut Ctx, account_id: &str) {
        self.reset(ctx);
        let key = format!("warm.{account_id}.home");
        ctx.store_read(&key, self.pending(Request::WarmHome));
        self.warm_key = Some(key);
    }

    /// Forgets everything; open surfaces stay open and show as loading.
    pub fn reset(&mut self, ctx: &mut Ctx) {
        if let Some(timer) = self.search.timer.take() {
            ctx.cancel_timer(timer);
        }
        let open = std::mem::take(&mut self.open);
        *self = Catalog { generation: self.generation + 1, open, ..Catalog::default() };
        self.render_all(ctx);
    }

    /// The display language changed: everything shown is in the old one, so it stays visible
    /// as stale while open surfaces reload.
    pub fn language_changed(&mut self, ctx: &mut Ctx, env: &Env) {
        self.generation += 1;
        self.home.invalidate();
        self.home.loading = false;
        self.genres = Slot { value: self.genres.value.take(), ..Slot::default() };
        self.my_list = Slot { value: self.my_list.value.take(), ..Slot::default() };
        for slot in self.titles.values_mut() {
            *slot = Slot { value: slot.value.take(), ..Slot::default() };
        }
        self.listings.clear();
        self.refresh_open(ctx, env);
        if !self.search.query.trim().is_empty() {
            self.run_search(ctx, env);
        }
        self.render_all(ctx);
    }

    pub fn opened(&mut self, ctx: &mut Ctx, env: Option<&Env>, surface: &Surface) {
        *self.open.entry(surface.clone()).or_default() += 1;
        if let Some(env) = env {
            self.ensure(ctx, env, surface, false);
        }
        ctx.render(surface.clone());
    }

    pub fn closed(&mut self, surface: &Surface) {
        if let Some(count) = self.open.get_mut(surface) {
            *count -= 1;
            if *count == 0 {
                self.open.remove(surface);
            }
        }
    }

    /// Back in the foreground: everything open may be out of date.
    pub fn refresh_open(&mut self, ctx: &mut Ctx, env: &Env) {
        let open: Vec<Surface> = self.open.keys().cloned().collect();
        for surface in open {
            self.ensure(ctx, env, &surface, true);
        }
    }

    /// Pull to refresh: reloads the surface even when it is fresh.
    pub fn refresh(&mut self, ctx: &mut Ctx, env: &Env, surface: &Surface) {
        if owns(surface) {
            self.ensure(ctx, env, surface, true);
            ctx.render(surface.clone());
        }
    }

    fn ensure(&mut self, ctx: &mut Ctx, env: &Env, surface: &Surface, force: bool) {
        let now = ctx.now;
        let lang = Some(env.language.to_string());
        match surface {
            Surface::Home if self.home.wants(now, force) => {
                self.home.loading = true;
                let call = ops::get_home(&GetHomeQuery { lang });
                self.http(ctx, env, &call.request.clone(), Request::Home(call));
            }
            Surface::Title(slug) if self.titles.entry(slug).wants(now, force) => {
                self.titles.entry(slug).loading = true;
                let call = ops::get_title(slug, &GetTitleQuery { lang });
                let request = call.request.clone();
                self.http(ctx, env, &request, Request::Title { slug: slug.clone(), call });
            }
            Surface::Browse(key) if self.listings.entry(key).slot.wants(now, force) => {
                self.listings.entry(key).slot.loading = true;
                self.browse_page(ctx, env, key, 1);
            }
            Surface::Genres if self.genres.wants(now, force) => {
                self.genres.loading = true;
                let call = ops::list_genres(&ListGenresQuery { lang });
                self.http(ctx, env, &call.request.clone(), Request::Genres(call));
            }
            Surface::MyList if self.my_list.wants(now, force) => {
                self.my_list.loading = true;
                let call = ops::list_watchlist(&ListWatchlistQuery { lang });
                self.http(ctx, env, &call.request.clone(), Request::MyList(call));
            }
            _ => {}
        }
    }

    fn browse_page(&mut self, ctx: &mut Ctx, env: &Env, key: &BrowseKey, page: u32) {
        let query = BrowseTitlesQuery {
            lang: Some(env.language.to_string()),
            kind: key.kind.map(|k| match k {
                TitleKind::Movie => BrowseTitlesKind::Movie,
                TitleKind::Series => BrowseTitlesKind::Series,
            }),
            genre: key.genre.clone(),
            q: None,
            sort: Some(key.sort.api()),
            page: Some(i64::from(page)),
        };
        let call = ops::browse_titles(&query);
        let request = call.request.clone();
        self.http(ctx, env, &request, Request::Browse { key: key.clone(), page, call });
    }

    pub fn browse_more(&mut self, ctx: &mut Ctx, env: &Env, key: &BrowseKey) {
        let Some(listing) = self.listings.get(key) else { return };
        let loaded = listing.slot.value.as_ref().map_or(0, Vec::len) as u64;
        if listing.slot.loading || listing.loading_more || loaded >= listing.total {
            return;
        }
        let page = listing.next_page;
        self.listings.entry(key).loading_more = true;
        self.browse_page(ctx, env, key, page);
        ctx.render(Surface::Browse(key.clone()));
    }

    pub fn search_changed(&mut self, ctx: &mut Ctx, query: &str) {
        self.search.query = query.to_string();
        self.search.seq += 1;
        if let Some(timer) = self.search.timer.take() {
            ctx.cancel_timer(timer);
        }
        if query.trim().is_empty() {
            self.search.slot = Slot::default();
        } else {
            self.search.slot.loading = true;
            let pending = self.pending(Request::SearchTimer { seq: self.search.seq });
            self.search.timer = Some(ctx.after(SEARCH_DEBOUNCE_MS, pending));
        }
        ctx.render(Surface::Search);
    }

    fn run_search(&mut self, ctx: &mut Ctx, env: &Env) {
        self.search.slot.loading = true;
        let query = SearchQuery {
            lang: Some(env.language.to_string()),
            q: Some(self.search.query.trim().to_string()),
        };
        let call = ops::search(&query);
        let request = call.request.clone();
        self.http(ctx, env, &request, Request::Search { seq: self.search.seq, call });
    }

    /// Adds or removes a title from My List at once, rolling back if the server refuses.
    pub fn set_listed(&mut self, ctx: &mut Ctx, env: &Env, title_id: &str, listed: bool) {
        self.listed.insert(title_id.to_string(), listed);
        let call = if listed {
            ops::add_to_watchlist(title_id)
        } else {
            ops::remove_from_watchlist(title_id)
        };
        let request = call.request.clone();
        self.http(
            ctx,
            env,
            &request,
            Request::Watchlist { title_id: title_id.to_string(), listed, call },
        );
        self.render_all(ctx);
    }

    fn http(
        &mut self,
        ctx: &mut Ctx,
        env: &Env,
        request: &couchverse_api::Request,
        pending: Request,
    ) {
        let pending = self.pending(pending);
        ctx.http(env.endpoint.request(request), pending);
    }

    fn pending(&self, request: Request) -> Pending {
        Pending::Catalog(CatalogPending { generation: self.generation, request })
    }

    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        env: Option<&Env>,
        pending: CatalogPending,
        output: EffectOutput,
    ) -> CatalogChange {
        if pending.generation != self.generation {
            return CatalogChange::None;
        }
        let now = ctx.now;
        let failure = match pending.request {
            Request::Home(call) => {
                ctx.render(Surface::Home);
                let result = decode(&call, output);
                if let Ok(home) = &result {
                    self.save_warm_home(ctx, home);
                }
                fill(&mut self.home, result, now)
            }
            Request::WarmHome => {
                self.warm_home(ctx, output);
                None
            }
            Request::Title { slug, call } => {
                ctx.render(Surface::Title(slug.clone()));
                fill(self.titles.entry(&slug), decode(&call, output), now)
            }
            Request::Browse { key, page, call } => {
                self.browsed(ctx, &key, page, decode(&call, output), now)
            }
            Request::Genres(call) => {
                ctx.render(Surface::Genres);
                fill(&mut self.genres, decode(&call, output), now)
            }
            Request::MyList(call) => {
                ctx.render(Surface::MyList);
                fill(&mut self.my_list, decode(&call, output), now)
            }
            Request::SearchTimer { seq } => {
                self.search.timer = None;
                if seq == self.search.seq
                    && let Some(env) = env
                {
                    self.run_search(ctx, env);
                }
                None
            }
            // an answer to a query the user has typed past is dropped
            Request::Search { seq, .. } if seq != self.search.seq => None,
            Request::Search { call, .. } => {
                ctx.render(Surface::Search);
                fill(&mut self.search.slot, decode(&call, output).map(|r| r.titles), now)
            }
            Request::Watchlist { title_id, listed, call } => {
                let result = decode(&call, output);
                if result.is_ok() {
                    self.watchlist_saved(&title_id, listed);
                }
                // confirmed or rolled back, the cached payloads now tell the truth
                self.listed.remove(&title_id);
                self.render_all(ctx);
                match result {
                    Ok(NoContent) => None,
                    Err(failure) if failure.unauthorized() => Some(failure),
                    Err(_) => return CatalogChange::WatchlistFailed,
                }
            }
        };
        match failure {
            Some(failure) if failure.unauthorized() => CatalogChange::Unauthorized,
            _ => CatalogChange::None,
        }
    }

    fn warm_home(&mut self, ctx: &mut Ctx, output: EffectOutput) {
        if let EffectOutput::Stored(stored) = output
            && self.home.value.is_none()
            && let Some(home) = stored.value.and_then(|j| serde_json::from_str(&j).ok())
        {
            // shown as stale until the network answers
            self.home.value = Some(home);
            ctx.render(Surface::Home);
        }
    }

    fn browsed(
        &mut self,
        ctx: &mut Ctx,
        key: &BrowseKey,
        page: u32,
        result: Result<BrowsePage, Failure>,
        now: U53,
    ) -> Option<Failure> {
        ctx.render(Surface::Browse(key.clone()));
        let listing = self.listings.entry(key);
        listing.loading_more = false;
        match result {
            Ok(result) => {
                let mut items =
                    if page == 1 { vec![] } else { listing.slot.value.take().unwrap_or_default() };
                items.extend(result.items);
                listing.total = u64::try_from(result.total).unwrap_or(0);
                listing.next_page = page + 1;
                if page == 1 {
                    listing.slot.loaded(items, now);
                } else {
                    listing.slot.value = Some(items);
                }
                None
            }
            // a failed first page fails the listing; a later one only reports it
            Err(failure) if page == 1 => {
                listing.slot.failed(&failure);
                Some(failure)
            }
            Err(failure) => {
                listing.slot.problem = Some(failure.problem());
                Some(failure)
            }
        }
    }

    /// Folds a confirmed My List change into the cached payloads.
    fn watchlist_saved(&mut self, title_id: &str, listed: bool) {
        for slot in self.titles.values_mut() {
            if let Some(detail) = slot.value.as_mut().filter(|d| d.title.id == title_id) {
                detail.in_watchlist = listed;
            }
        }
        if let Some(home) = self.home.value.as_mut() {
            for item in home.featured.iter_mut().filter(|f| f.id == title_id) {
                item.in_list = listed;
            }
        }
        match (listed, self.my_list.value.as_mut()) {
            (false, Some(items)) => items.retain(|i| i.title_id != title_id),
            // the new card's details are not at hand; the list reloads when next shown
            _ => self.my_list.invalidate(),
        }
        if listed {
            self.home.invalidate();
        }
    }

    fn save_warm_home(&self, ctx: &mut Ctx, home: &Home) {
        if let Some(key) = &self.warm_key
            && let Ok(json) = serde_json::to_string(home)
        {
            ctx.store_write(key, json);
        }
    }

    fn render_all(&self, ctx: &mut Ctx) {
        ctx.render(Surface::Home);
        ctx.render(Surface::Genres);
        ctx.render(Surface::MyList);
        ctx.render(Surface::Search);
        for surface in self.open.keys() {
            ctx.render(surface.clone());
        }
    }

    pub fn home_view(&self, images: &Images) -> HomeView {
        views::home(
            self.home.status(),
            self.home.value.as_ref(),
            self.home.problem.clone(),
            images,
            &self.listed,
        )
    }

    pub fn title_view(&self, slug: &str, images: &Images) -> TitleView {
        let slot = self.titles.get(&slug.to_string());
        TitleView {
            slug: slug.to_string(),
            status: slot.map_or(LoadStatus::Idle, Slot::status),
            detail: slot
                .and_then(|s| s.value.as_ref())
                .map(|d| views::title(d, images, &self.listed)),
            problem: slot.and_then(|s| s.problem.clone()),
        }
    }

    pub fn browse_view(&self, key: &BrowseKey, images: &Images) -> BrowseView {
        let listing = self.listings.get(key);
        let cards = listing
            .and_then(|l| l.slot.value.as_deref())
            .map(|items| views::cards(items, images))
            .unwrap_or_default();
        let total = listing.map_or(0, |l| l.total);
        BrowseView {
            key: key.clone(),
            status: listing.map_or(LoadStatus::Idle, |l| l.slot.status()),
            more: (cards.len() as u64) < total,
            cards,
            total,
            loading_more: listing.is_some_and(|l| l.loading_more),
            problem: listing.and_then(|l| l.slot.problem.clone()),
        }
    }

    pub fn genres_view(&self) -> GenresView {
        views::genres(
            self.genres.status(),
            self.genres.value.as_deref(),
            self.genres.problem.clone(),
        )
    }

    pub fn my_list_view(&self, images: &Images) -> MyListView {
        let items = self.my_list.value.as_deref().unwrap_or_default();
        MyListView {
            status: self.my_list.status(),
            cards: items
                .iter()
                .filter(|i| self.listed.get(&i.title_id) != Some(&false))
                .map(|i| views::card(i, images))
                .collect(),
            problem: self.my_list.problem.clone(),
        }
    }

    pub fn search_view(&self, images: &Images) -> SearchView {
        SearchView {
            query: self.search.query.clone(),
            status: self.search.slot.status(),
            cards: views::cards(self.search.slot.value.as_deref().unwrap_or_default(), images),
            problem: self.search.slot.problem.clone(),
        }
    }
}
