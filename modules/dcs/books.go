// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import "strconv"

var BookNames = map[string]string{ //nolint
	"frt": "Front Matter",
	"bak": "Back Matter",
	"gen": "Genesis",
	"exo": "Exodus",
	"lev": "Leviticus",
	"num": "Numbers",
	"deu": "Deuteronomy",
	"jos": "Joshua",
	"jdg": "Judges",
	"rut": "Ruth",
	"1sa": "1 Samuel",
	"2sa": "2 Samuel",
	"1ki": "1 Kings",
	"2ki": "2 Kings",
	"1ch": "1 Chronicles",
	"2ch": "2 Chronicles",
	"ezr": "Ezra",
	"neh": "Nehemiah",
	"est": "Esther",
	"job": "Job",
	"psa": "Psalms",
	"pro": "Proverbs",
	"ecc": "Ecclesiastes",
	"sng": "Song of Solomon",
	"isa": "Isaiah",
	"jer": "Jeremiah",
	"lam": "Lamentations",
	"ezk": "Ezekiel",
	"dan": "Daniel",
	"hos": "Hosea",
	"jol": "Joel",
	"amo": "Amos",
	"oba": "Obadiah",
	"jon": "Jonah",
	"mic": "Micah",
	"nam": "Nahum",
	"hab": "Habakkuk",
	"zep": "Zephaniah",
	"hag": "Haggai",
	"zec": "Zechariah",
	"mal": "Malachi",
	"mat": "Matthew",
	"mrk": "Mark",
	"luk": "Luke",
	"jhn": "John",
	"act": "Acts",
	"rom": "Romans",
	"1co": "1 Corinthians",
	"2co": "2 Corinthians",
	"gal": "Galatians",
	"eph": "Ephesians",
	"php": "Philippians",
	"col": "Colossians",
	"1th": "1 Thessalonians",
	"2th": "2 Thessalonians",
	"1ti": "1 Timothy",
	"2ti": "2 Timothy",
	"tit": "Titus",
	"phm": "Philemon",
	"heb": "Hebrews",
	"jas": "James",
	"1pe": "1 Peter",
	"2pe": "2 Peter",
	"1jn": "1 John",
	"2jn": "2 John",
	"3jn": "3 John",
	"jud": "Jude",
	"rev": "Revelation",
	"obs": "Open Bible Stories",
}

var BookNumbers = map[string]string{ //nolint
	"frt": "A0",
	"bak": "B0",
	"gen": "01",
	"exo": "02",
	"lev": "03",
	"num": "04",
	"deu": "05",
	"jos": "06",
	"jdg": "07",
	"rut": "08",
	"1sa": "09",
	"2sa": "10",
	"1ki": "11",
	"2ki": "12",
	"1ch": "13",
	"2ch": "14",
	"ezr": "15",
	"neh": "16",
	"est": "17",
	"job": "18",
	"psa": "19",
	"pro": "20",
	"ecc": "21",
	"sng": "22",
	"isa": "23",
	"jer": "24",
	"lam": "25",
	"ezk": "26",
	"dan": "27",
	"hos": "28",
	"jol": "29",
	"amo": "30",
	"oba": "31",
	"jon": "32",
	"mic": "33",
	"nam": "34",
	"hab": "35",
	"zep": "36",
	"hag": "37",
	"zec": "38",
	"mal": "39",
	"mat": "41",
	"mrk": "42",
	"luk": "43",
	"jhn": "44",
	"act": "45",
	"rom": "46",
	"1co": "47",
	"2co": "48",
	"gal": "49",
	"eph": "50",
	"php": "51",
	"col": "52",
	"1th": "53",
	"2th": "54",
	"1ti": "55",
	"2ti": "56",
	"tit": "57",
	"phm": "58",
	"heb": "59",
	"jas": "60",
	"1pe": "61",
	"2pe": "62",
	"1jn": "63",
	"2jn": "64",
	"3jn": "65",
	"jud": "66",
	"rev": "67",
	"obs": "0",
}

// BookIsValid returns true if string is a valid book or is obs
func BookIsValid(book string) bool {
	_, ok := BookNames[book]
	return ok
}

func BookIsOT(book string) bool {
	return BookIsValid(book) && BookNumbers[book] > "0" && BookNumbers[book] < "40"
}

func BookIsNT(book string) bool {
	return BookIsValid(book) && BookNumbers[book] > "40" && BookNumbers[book] <= "67"
}

func GetTestament(book string) string {
	if BookIsOT(book) {
		return "ot"
	}
	if BookIsNT(book) {
		return "nt"
	}
	return ""
}

func GetBookCategories(book string) []string {
	testament := GetTestament(book)
	if testament != "" {
		testament = "bible-" + testament
		return []string{testament}
	}
	return nil
}

func GetBookSort(book string) int {
	var i int
	if num, ok := BookNumbers[book]; ok {
		i, _ = strconv.Atoi(num)
	}
	return i
}
