import tempfile
import unittest
from pathlib import Path

from couchpush.api.models import Title
from couchpush.core.matcher import (
    ACTION_SKIP,
    build_movie_row,
    build_movie_rows,
    movie_selection_errors,
    movie_target_key,
    suggest_movies,
)
from couchpush.core.parser import MovieFile, parse_movie, scan_movies


def title(name, year=None, id="movie", has_media=False, kind="movie"):
    return Title.from_json({
        "id": id, "name": name, "year": year, "kind": kind,
        "sizeBytes": 1024 if has_media else 0,
    })


def movie(name, year=None, path="source.mkv"):
    return MovieFile(path=path, rel_path=path, name=name, year=year)


class MovieParserTests(unittest.TestCase):
    def test_iron_man_release_filenames(self):
        cases = {
            "Iron Man 2 (2010) UHDRDV cz en.mkv": ("Iron Man 2", 2010),
            "Iron.Man.2008.UHDRDV.cz.en.mkv": ("Iron Man", 2008),
            "Iron_Man_3_[2013]_2160p_HDR10_DV.mkv": ("Iron Man 3", 2013),
            "Iron Man 2 Blu-Ray x265.mkv": ("Iron Man 2", None),
            "Iron Man 2 HDR cz en.mkv": ("Iron Man 2", None),
        }
        for filename, expected in cases.items():
            with self.subTest(filename=filename):
                self.assertEqual(parse_movie(filename), expected)

    def test_nearest_movie_folder_supplies_title_and_year(self):
        self.assertEqual(
            parse_movie(r"Collection\Marvel\Iron Man 2 (2010)\Feature\movie.mkv"),
            ("Iron Man 2", 2010),
        )
        self.assertEqual(parse_movie("Marvel/Iron Man/video.mkv"), ("Iron Man", None))

    def test_numbers_and_unicode_in_titles_survive(self):
        self.assertEqual(parse_movie("2001.A.Space.Odyssey.(1968).mkv"),
                         ("2001 A Space Odyssey", 1968))
        self.assertEqual(parse_movie("Blade Runner 2049.mkv"), ("Blade Runner 2049", None))
        self.assertEqual(parse_movie("1917.mkv"), ("1917", None))
        self.assertEqual(parse_movie("Pelíšky (1999).mkv"), ("Pelíšky", 1999))

    def test_scan_keeps_movie_sources_and_ignores_sidecars(self):
        with tempfile.TemporaryDirectory() as root:
            folder = Path(root, "Collection", "Iron Man 2 (2010)")
            folder.mkdir(parents=True)
            Path(folder, "movie.MKV").touch()
            Path(folder, "movie.en.vtt").touch()
            found = scan_movies(root)
            self.assertEqual(len(found), 1)
            self.assertEqual((found[0].name, found[0].year), ("Iron Man 2", 2010))
            self.assertTrue(Path(found[0].path).is_absolute())


class MovieMatcherTests(unittest.TestCase):
    def test_wrong_year_remake_creates_a_separate_movie(self):
        row = build_movie_rows([title("Dune", 1984)], [movie("Dune", 2021)])[0]
        self.assertIsNone(row.title_id)
        self.assertTrue(row.selected)
        self.assertFalse(row.needs_review)
        self.assertEqual(row.output_base, "Dune (2021)")

    def test_exact_year_wins_over_remakes_and_missing_year(self):
        rows = build_movie_rows(
            [title("Dune", 1984, "old"), title("Dune", None, "unknown"),
             title("Dune", 2021, "new")],
            [movie("Dune", 2021)],
        )
        self.assertEqual(rows[0].title_id, "new")
        self.assertTrue(rows[0].selected)

    def test_ambiguous_release_requires_explicit_assignment(self):
        cases = [
            ([title("Dune", 1984, "old"), title("Dune", 2021, "new")], movie("Dune")),
            ([title("Dune", 2021, "a"), title("Dune", 2021, "b")], movie("Dune", 2021)),
            ([title("Dune", None)], movie("Dune", 2021)),
        ]
        for catalog, source in cases:
            with self.subTest(catalog=catalog):
                row = build_movie_rows(catalog, [source])[0]
                self.assertTrue(row.needs_review)
                self.assertFalse(row.selected)
                self.assertEqual(movie_selection_errors([row]), [])
                row.selected = True
                self.assertIn("choose a movie target", movie_selection_errors([row])[0])

    def test_unicode_titles_do_not_collapse_to_empty_ascii(self):
        catalog = [title("天空の城ラピュタ", 1986, "laputa"),
                   title("千と千尋の神隠し", 2001, "spirited")]
        row = build_movie_rows(catalog, [movie("千と千尋の神隠し", 2001)])[0]
        self.assertEqual(row.title_id, "spirited")
        accented = build_movie_rows([title("Pelíšky", 1999)], [movie("Pelisky", 1999)])[0]
        self.assertEqual(accented.title_id, "movie")

    def test_existing_media_defaults_to_skip(self):
        row = build_movie_rows([title("Iron Man 2", 2010, has_media=True)],
                               [movie("Iron Man 2", 2010)])[0]
        self.assertTrue(row.has_media)
        self.assertFalse(row.selected)
        self.assertEqual(row.action, ACTION_SKIP)

    def test_existing_target_duplicates_ignore_source_year(self):
        rows = build_movie_rows([title("Iron Man 2", 2010)],
                                [movie("Iron Man 2", 2010, "a.mkv"),
                                 movie("Iron Man 2", None, "b.mkv")])
        self.assertEqual(rows[1].duplicate_of, 0)
        self.assertFalse(rows[1].selected)
        rows[1].selected = True
        self.assertIn("Select only one file", movie_selection_errors(rows)[0])

    def test_manual_target_resolves_ambiguity_and_unparsed_source(self):
        source = movie("", None)
        row = build_movie_row(source, title("Iron Man 2", 2010))
        self.assertFalse(source.ok)
        self.assertFalse(row.needs_review)
        self.assertTrue(row.selected)
        self.assertEqual(movie_selection_errors([row]), [])
        self.assertEqual(row.output_base, "Iron Man 2 (2010)")

    def test_new_title_edit_keeps_source_and_uses_corrected_metadata(self):
        source = movie("Bad Filename", 2020)
        row = build_movie_row(source, new_name="Iron Man 2", new_year=2010)
        self.assertIs(row.movie, source)
        self.assertEqual(source.name, "Bad Filename")
        self.assertEqual(row.output_base, "Iron Man 2 (2010)")
        self.assertEqual(row.target_title, "Iron Man 2")
        self.assertEqual(row.target_year, 2010)
        no_year = build_movie_row(source, new_name="Iron Man 2", new_year=None)
        self.assertEqual(no_year.output_base, "Iron Man 2")
        self.assertIsNone(no_year.target_year)

    def test_manual_new_titles_detect_duplicates_after_edits(self):
        rows = [build_movie_row(movie("a", path="a.mkv"), new_name="Pelíšky", new_year=1999),
                build_movie_row(movie("b", path="b.mkv"), new_name="Pelisky", new_year=1999)]
        self.assertEqual(movie_target_key(rows[0]), movie_target_key(rows[1]))
        self.assertEqual(len(movie_selection_errors(rows)), 1)
        rows[1].target_year = 2000
        self.assertEqual(movie_selection_errors(rows), [])

    def test_manual_existing_target_duplicates_ignore_parsed_year(self):
        target = title("Dune", 2021)
        rows = [build_movie_row(movie("Dune", 1984, "a.mkv"), target),
                build_movie_row(movie("Dune", 2021, "b.mkv"), target)]
        self.assertEqual(len(movie_selection_errors(rows)), 1)

    def test_fuzzy_suggestions_never_automatically_attach(self):
        source = movie("IronMan 2", 2010)
        catalog = [title("Unrelated", 2010, "other"), title("Iron Man 2", 2010, "iron")]
        self.assertIsNone(build_movie_rows(catalog, [source])[0].title_id)
        self.assertEqual(suggest_movies(source, catalog)[0].id, "iron")

    def test_series_titles_cannot_be_assigned_as_movies(self):
        series = title("Iron Man", 1994, kind="series")
        self.assertIsNone(build_movie_rows([series], [movie("Iron Man", 1994)])[0].title_id)
        with self.assertRaises(ValueError):
            build_movie_row(movie("Iron Man", 1994), series)


if __name__ == "__main__":
    unittest.main()
