import Foundation
import Testing

@testable import CouchverseCore

/// Ranks and profile editing through the runtime over the real core.
extension RuntimeTests {
    /// A phone signed in as the admin on `Payload.storedServer`, its session loaded.
    func signedInRuntime() async throws -> CoreRuntime {
        let runtime = try makeRuntime(store: MemoryStore(Payload.signedIn))
        try secure.write("token.\(Payload.accountId)", value: "tok-1")
        Payload.routeSession(http, base: "http://tv.home", user: Payload.user(1, "admin"))
        runtime.start()
        await runtime.untilIdle()
        return runtime
    }

    @Test func profilesArePublishedByUsernameOnceTheirScreenOpens() async throws {
        let runtime = try await signedInRuntime()
        #expect(runtime.profile("admin").status == .loading, "not open yet")
        http.route("GET", "http://tv.home/api/v1/me/stats?lang=cs", Payload.profile("admin", isSelf: true))

        runtime.send(.screenOpened(.profile("admin")))
        await runtime.untilIdle()

        let profile = try #require(runtime.profile("admin").profile)
        #expect(profile.isSelf)
        #expect(profile.rank.tier.code == "remote")
        #expect(profile.hours.count == 24)
        #expect(runtime.rank.rank?.tier.level == 2, "your own profile carries your rank")
        #expect(runtime.profile("nora").status == .loading)
    }

    @Test func everyMetricOfABoardIsPublishedFromOneAnswer() async throws {
        let runtime = try await signedInRuntime()
        http.route("GET", "http://tv.home/api/v1/leaderboard?period=week", Payload.board)
        let xp = LeaderboardKey(period: .week, metric: .xp)

        runtime.send(.screenOpened(.leaderboard(xp)))
        await runtime.untilIdle()

        #expect(runtime.leaderboard(xp).rows.map(\.username) == ["nora", "admin"])
        let watch = runtime.leaderboard(LeaderboardKey(period: .week, metric: .watch))
        #expect(watch.rows.map(\.username) == ["admin", "nora"])
        #expect(watch.me?.position == 1)
        #expect(runtime.leaderboard(LeaderboardKey(period: .all, metric: .xp)).status == .loading)
    }

    @Test func aPickedImageIsUploadedAndTheNewAvatarShows() async throws {
        let runtime = try await signedInRuntime()
        http.route(
            "POST", "http://tv.home/api/v1/me/avatar",
            Payload.user(1, "admin").replacingOccurrences(of: "av-1", with: "av-new"))

        runtime.send(.imageChosen(ImageChoice(slot: .avatar, file: "file:///tmp/uploads/pick.jpg")))
        await runtime.untilIdle()

        let upload = try #require(http.uploads.first)
        #expect(upload.file == "file:///tmp/uploads/pick.jpg")
        #expect(upload.field == "file")
        #expect(upload.request.headers.contains(HttpHeader(name: "Authorization", value: "Bearer tok-1")))
        #expect(runtime.profileEditor.avatar.status == .loaded)
        #expect(runtime.session.user?.avatarId == "av-new")
    }

    @Test func aFailedUploadIsTheEditorsToShow() async throws {
        let runtime = try await signedInRuntime()

        runtime.send(.imageChosen(ImageChoice(slot: .banner, file: "file:///tmp/uploads/pick.jpg")))
        await runtime.untilIdle()

        #expect(runtime.profileEditor.banner.status == .failed)
        #expect(runtime.profileEditor.banner.problem?.code == "offline")
        #expect(runtime.app.phase == .ready)
    }
}

extension Payload {
    static func profile(_ username: String, isSelf: Bool) -> String {
        let tier = { (code: String, level: Int, minXp: Int) in
            ##"{"code":"\##(code)","level":\##(level),"colour":"#60a5fa","minXp":\##(minXp)}"##
        }
        let achievement = { (code: String, unlocked: Bool) in
            """
            {"code":"\(code)","category":"watching","tier":"bronze","unlocked":\(unlocked),\
            "unlockedAt":\(unlocked ? #""2026-10-02T20:00:00Z""# : "null"),"value":1,"target":1,\
            "percent":100,"xp":50}
            """
        }
        return """
            {"user":{"username":"\(username)","displayName":"\(username.capitalized)","bio":"Mostly **sci-fi**",\
            "avatarId":"av","bannerId":null,"bannerAccent":"","memberSince":"2025-01-01T00:00:00Z"},\
            "isSelf":\(isSelf),"public":true,\
            "rank":{"tier":\(tier("remote", 2, 500)),"next":\(tier("snack", 3, 1500)),"xp":640,"percent":14,\
            "intoTier":140,"tierSpan":1000},\
            "xp":{"total":640,"sources":[{"key":"video","units":220,"rate":2,"xp":440}]},\
            "achievements":[\(achievement("first_play", true)),\(achievement("watch_10h", false))],\
            "achievementsWon":1,"recentUnlocks":[\(achievement("first_play", true))],\
            "totals":{"videoSeconds":5400,"moviesCompleted":1,"episodesCompleted":2,"seriesCompleted":0,\
            "distinctTitles":2,"distinctGenres":3,"activeDays":2,"currentStreak":1,"longestStreak":2,\
            "bestDayMinutes":60,"couchHosted":0,"couchJoined":1,"biggestCouch":2,"emojiSent":4},\
            "topTitles":[],"favouriteGenre":"Drama","hours":[{"hour":21,"videoSeconds":3600}],\
            "activity":{"from":"2026-09-30","days":[0,1800,3600]}}
            """
    }

    static let board: String = {
        let row = { (name: String, xp: Int, watch: Int, me: Bool) in
            """
            {"username":"\(name)","displayName":"\(name.capitalized)","avatarId":null,"level":2,\
            "tierCode":"remote","xp":\(xp),"watchSeconds":\(watch),"achievements":3,"isSelf":\(me)}
            """
        }
        return """
            {"period":"week","hidden":false,"total":2,"me":\(row("admin", 300, 9000, true)),\
            "rows":[\(row("admin", 300, 9000, true)),\(row("nora", 900, 3000, false))]}
            """
    }()
}
