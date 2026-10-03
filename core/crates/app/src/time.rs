//! Wall-clock time as text. The core has no clock: shells stamp messages with milliseconds since
//! the Unix epoch where an answer depends on the date (offline progress), and the API speaks
//! RFC 3339.

/// Seconds since the epoch for an RFC 3339 timestamp (`2026-10-02T12:00:00.123+02:00`).
pub fn unix_seconds(timestamp: &str) -> Option<i64> {
    let (date, rest) = timestamp.split_once(['T', 't', ' '])?;
    let mut date = date.splitn(3, '-').map(str::parse::<i64>);
    let (year, month, day) = (date.next()?.ok()?, date.next()?.ok()?, date.next()?.ok()?);
    let time_end = rest.find(['Z', 'z', '+', '-']).unwrap_or(rest.len());
    let (time, zone) = rest.split_at(time_end);
    let time = time.split('.').next()?;
    let mut time = time.splitn(3, ':').map(str::parse::<i64>);
    let (hour, minute, second) = (time.next()?.ok()?, time.next()?.ok()?, time.next()?.ok()?);
    let offset = match zone.chars().next() {
        Some(sign @ ('+' | '-')) => {
            let (h, m) = zone[1..].split_once(':')?;
            let minutes = h.parse::<i64>().ok()? * 60 + m.parse::<i64>().ok()?;
            if sign == '+' { minutes * 60 } else { -minutes * 60 }
        }
        _ => 0,
    };
    Some(days_from_civil(year, month, day) * 86_400 + hour * 3600 + minute * 60 + second - offset)
}

/// Days since 1970-01-01 in the proleptic Gregorian calendar (Howard Hinnant's algorithm).
fn days_from_civil(year: i64, month: i64, day: i64) -> i64 {
    let year = if month <= 2 { year - 1 } else { year };
    let era = year.div_euclid(400);
    let year_of_era = year - era * 400;
    let month_index = (month + 9) % 12;
    let day_of_year = (153 * month_index + 2) / 5 + day - 1;
    let day_of_era = year_of_era * 365 + year_of_era / 4 - year_of_era / 100 + day_of_year;
    era * 146_097 + day_of_era - 719_468
}

/// The RFC 3339 form of milliseconds since the Unix epoch, in UTC (`2026-10-02T12:00:00Z`).
pub fn rfc3339(ms: u64) -> String {
    let seconds = i64::try_from(ms / 1000).unwrap_or(i64::MAX);
    let (days, rest) = (seconds.div_euclid(86_400), seconds.rem_euclid(86_400));
    let (year, month, day) = civil_from_days(days);
    format!(
        "{year:04}-{month:02}-{day:02}T{:02}:{:02}:{:02}Z",
        rest / 3600,
        rest % 3600 / 60,
        rest % 60
    )
}

/// The date of a day since 1970-01-01 (Howard Hinnant's algorithm, the inverse of
/// `days_from_civil`).
fn civil_from_days(days: i64) -> (i64, i64, i64) {
    let z = days + 719_468;
    let era = z.div_euclid(146_097);
    let day_of_era = z - era * 146_097;
    let year_of_era =
        (day_of_era - day_of_era / 1460 + day_of_era / 36_524 - day_of_era / 146_096) / 365;
    let day_of_year = day_of_era - (365 * year_of_era + year_of_era / 4 - year_of_era / 100);
    let month_index = (5 * day_of_year + 2) / 153;
    let day = day_of_year - (153 * month_index + 2) / 5 + 1;
    let month = if month_index < 10 { month_index + 3 } else { month_index - 9 };
    let year = year_of_era + era * 400 + i64::from(month <= 2);
    (year, month, day)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn timestamps_become_unix_seconds() {
        assert_eq!(unix_seconds("1970-01-01T00:00:00Z"), Some(0));
        assert_eq!(unix_seconds("2026-10-02T12:00:00Z"), Some(1_790_942_400));
        assert_eq!(unix_seconds("2026-10-02T14:00:00.987654+02:00"), Some(1_790_942_400));
        assert_eq!(unix_seconds("2000-02-29T23:59:59-01:30"), Some(951_874_199));
        assert_eq!(unix_seconds("not a date"), None);
    }

    #[test]
    fn milliseconds_become_rfc3339() {
        assert_eq!(rfc3339(0), "1970-01-01T00:00:00Z");
        assert_eq!(rfc3339(1_790_942_400_999), "2026-10-02T12:00:00Z");
        assert_eq!(rfc3339(951_874_199_000), "2000-03-01T01:29:59Z");
        for seconds in [0, 951_782_400, 1_790_942_400, 4_102_444_799] {
            let text = rfc3339(seconds * 1000);
            assert_eq!(unix_seconds(&text), Some(i64::try_from(seconds).unwrap()), "{text}");
        }
    }
}
