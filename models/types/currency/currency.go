package currency

import (
	"strconv"
	"strings"
)

// import (
// 	"github.com/mholt/binding"
// 	"net/http"
// )
// type Currency struct {
// 	value int64
// 	FieldMapMixin
// }

// func (c Currency) Validate(req *http.Request, errs binding.Errors) binding.Errors {
// 	return errs
// }

// func (c Currency) Add()    {}
// func (c Currency) Sub()    {}
// func (c Currency) Mul()    {}
// func (c Currency) String() {}

type Cents int

type Type string

func (t Type) Symbol() string {
	switch t {
	case ARS, AUD, BSD, BBD, BMD, BND, CAD, KYD, CLP, COP, XCD, SVC, FJD, GYD, HKD, LRD, MXN, NAD, NZD, SGD, SBD, SRD, USD:
		return "$"
	case BOB:
		return "$b"
	case UYU:
		return "$U"
	case EGP, FKP, GIP, LBP, SHP, GBP:
		return "£"
	case CNY, JPY:
		return "¥"
	case AFN:
		return "؋"
	case THB:
		return "฿"
	case KHR:
		return "៛"
	case CRC:
		return "₡"
	case NGN:
		return "₦"
	case KRW:
		return "₩"
	case ILS:
		return "₪"
	case VND:
		return "₫"
	case EUR:
		return "€"
	case LAK:
		return "₭"
	case MNT:
		return "₮"
	case PHP:
		return "₱"
	case UAH:
		return "₴"
	case MUR, NPR, PKR, SCR, LKR:
		return "₨"
	case QAR, SAR, YER:
		return "﷼"
	case PAB:
		return "B/."
	case BZD:
		return "BZ$"
	case NIO:
		return "C$"
	case CHF:
		return "CHF"
	case HUF:
		return "Ft"
	case AWG, ANG:
		return "ƒ"
	case PYG:
		return "Gs"
	case JMD:
		return "J$"
	case CZK:
		return "Kč"
	case BAM:
		return "KM"
	case HRK, DKK, EEK, ISK, NOK, SEK:
		return "kr"
	case HNL:
		return "L"
	case RON:
		return "lei"
	case ALL:
		return "Lek"
	case LVL:
		return "Ls"
	case LTL:
		return "Lt"
	case MZN:
		return "MT"
	case TWD:
		return "NT$"
	case BWP:
		return "P"
	case GTQ:
		return "Q"
	case ZAR:
		return "R"
	case BRL:
		return "R$"
	case DOP:
		return "RD$"
	case MYR:
		return "RM"
	case IDR:
		return "Rp"
	case SOS:
		return "S"
	case PEN:
		return "S/."
	case TTD:
		return "TT$"
	case PLN:
		return "zł"
	case MKD:
		return "ден"
	case RSD:
		return "Дин."
	case BGN, KZT, KGS, UZS:
		return "лв"
	case AZN:
		return "ман"
	case RUB:
		return "руб"
	case INR, TRY:
		return ""
	}
	return ""
}

func (t Type) IsZeroDecimal() bool {
	switch t {
	case BIF, CLP, DJF, GNF, JPY, KMF, KRW, MGA, PYG, RWF, VND, VUV, XAF, XOF, XPF:
		return true
	}

	return false
}

func (t Type) ToString(c Cents) string {
	if t.IsZeroDecimal() {
		return t.Symbol() + strconv.Itoa(int(c))
	}
	cents := strconv.Itoa(int(c) % 100)
	if len(cents) < 2 {
		cents += "0"
	}
	return t.Symbol() + strconv.Itoa(int(c)/100) + "." + cents
}

func (t Type) Label() string {
	return t.Symbol() + " " + t.Code()
}

func (t Type) Code() string {
	return strings.ToUpper(string(t))
}

const (
	USD Type = "usd"
	AUD      = "aud"
	CAD      = "cad"
	EUR      = "eur"
	GBP      = "gbp"
	HKD      = "hkd"
	JPY      = "jpy"
	NZD      = "nzd"
	SGD      = "sgd"
	AED      = "aed" // United Arab Emirates Dirham
	AFN      = "afn" // Afghan Afghani*
	ALL      = "all" // Albanian Lek
	AMD      = "amd" // Armenian Dram
	ANG      = "ang" // Netherlands Antillean Gulden
	AOA      = "aoa" // Angolan Kwanza*
	ARS      = "ars" // Argentine Peso*
	AWG      = "awg" // Aruban Florin
	AZN      = "azn" // Azerbaijani Manat
	BAM      = "bam" // Bosnia & Herzegovina Convertible Mark
	BBD      = "bbd" // Barbadian Dollar
	BDT      = "bdt" // Bangladeshi Taka
	BGN      = "bgn" // Bulgarian Lev
	BIF      = "bif" // Burundian Franc
	BMD      = "bmd" // Bermudian Dollar
	BND      = "bnd" // Brunei Dollar
	BOB      = "bob" // Bolivian Boliviano*
	BRL      = "brl" // Brazilian Real*
	BSD      = "bsd" // Bahamian Dollar
	BWP      = "bwp" // Botswana Pula
	BZD      = "bzd" // Belize Dollar
	CDF      = "cdf" // Congolese Franc
	CHF      = "chf" // Swiss Franc
	CLP      = "clp" // Chilean Peso*
	CNY      = "cny" // Chinese Renminbi Yuan
	COP      = "cop" // Colombian Peso*
	CRC      = "crc" // Costa Rican Colón*
	CVE      = "cve" // Cape Verdean Escudo*
	CZK      = "czk" // Czech Koruna*
	DJF      = "djf" // Djiboutian Franc*
	DKK      = "dkk" // Danish Krone
	DOP      = "dop" // Dominican Peso
	DZD      = "dzd" // Algerian Dinar
	EEK      = "eek" // Estonian Kroon*
	EGP      = "egp" // Egyptian Pound
	ETB      = "etb" // Ethiopian Birr
	FJD      = "fjd" // Fijian Dollar
	FKP      = "fkp" // Falkland Islands Pound*
	GEL      = "gel" // Georgian Lari
	GIP      = "gip" // Gibraltar Pound
	GMD      = "gmd" // Gambian Dalasi
	GNF      = "gnf" // Guinean Franc*
	GTQ      = "gtq" // Guatemalan Quetzal*
	GYD      = "gyd" // Guyanese Dollar
	HNL      = "hnl" // Honduran Lempira*
	HRK      = "hrk" // Croatian Kuna
	HTG      = "htg" // Haitian Gourde
	HUF      = "huf" // Hungarian Forint*
	IDR      = "idr" // Indonesian Rupiah
	ILS      = "ils" // Israeli New Sheqel
	INR      = "inr" // Indian Rupee*
	ISK      = "isk" // Icelandic Króna
	JMD      = "jmd" // Jamaican Dollar
	KES      = "kes" // Kenyan Shilling
	KGS      = "kgs" // Kyrgyzstani Som
	KHR      = "khr" // Cambodian Riel
	KMF      = "kmf" // Comorian Franc
	KRW      = "krw" // South Korean Won
	KYD      = "kyd" // Cayman Islands Dollar
	KZT      = "kzt" // Kazakhstani Tenge
	LAK      = "lak" // Lao Kip*
	LBP      = "lbp" // Lebanese Pound
	LKR      = "lkr" // Sri Lankan Rupee
	LRD      = "lrd" // Liberian Dollar
	LSL      = "lsl" // Lesotho Loti
	LTL      = "ltl" // Lithuanian Litas
	LVL      = "lvl" // Latvian Lats
	MAD      = "mad" // Moroccan Dirham
	MDL      = "mdl" // Moldovan Leu
	MGA      = "mga" // Malagasy Ariary
	MKD      = "mkd" // Macedonian Denar
	MNT      = "mnt" // Mongolian Tögrög
	MOP      = "mop" // Macanese Pataca
	MRO      = "mro" // Mauritanian Ouguiya
	MUR      = "mur" // Mauritian Rupee*
	MVR      = "mvr" // Maldivian Rufiyaa
	MWK      = "mwk" // Malawian Kwacha
	MXN      = "mxn" // Mexican Peso*
	MYR      = "myr" // Malaysian Ringgit
	MZN      = "mzn" // Mozambican Metical
	NAD      = "nad" // Namibian Dollar
	NGN      = "ngn" // Nigerian Naira
	NIO      = "nio" // Nicaraguan Córdoba*
	NOK      = "nok" // Norwegian Krone
	NPR      = "npr" // Nepalese Rupee
	PAB      = "pab" // Panamanian Balboa*
	PEN      = "pen" // Peruvian Nuevo Sol*
	PGK      = "pgk" // Papua New Guinean Kina
	PHP      = "php" // Philippine Peso
	PKR      = "pkr" // Pakistani Rupee
	PLN      = "pln" // Polish Złoty
	PYG      = "pyg" // Paraguayan Guaraní*
	QAR      = "qar" // Qatari Riyal
	RON      = "ron" // Romanian Leu
	RSD      = "rsd" // Serbian Dinar
	RUB      = "rub" // Russian Ruble
	RWF      = "rwf" // Rwandan Franc
	SAR      = "sar" // Saudi Riyal
	SBD      = "sbd" // Solomon Islands Dollar
	SCR      = "scr" // Seychellois Rupee
	SEK      = "sek" // Swedish Krona
	SHP      = "shp" // Saint Helenian Pound*
	SLL      = "sll" // Sierra Leonean Leone
	SOS      = "sos" // Somali Shilling
	SRD      = "srd" // Surinamese Dollar*
	STD      = "std" // São Tomé and Príncipe Dobra
	SVC      = "svc" // Salvadoran Colón*
	SZL      = "szl" // Swazi Lilangeni
	THB      = "thb" // Thai Baht
	TJS      = "tjs" // Tajikistani Somoni
	TOP      = "top" // Tongan Paʻanga
	TRY      = "try" // Turkish Lira
	TTD      = "ttd" // Trinidad and Tobago Dollar
	TWD      = "twd" // New Taiwan Dollar
	TZS      = "tzs" // Tanzanian Shilling
	UAH      = "uah" // Ukrainian Hryvnia
	UGX      = "ugx" // Ugandan Shilling
	UYU      = "uyu" // Uruguayan Peso*
	UZS      = "uzs" // Uzbekistani Som
	VND      = "vnd" // Vietnamese Đồng
	VUV      = "vuv" // Vanuatu Vat
	UWT      = "uwt" // Samoan Tala
	XAF      = "xaf" // Central African Cfa Franc
	XCD      = "xcd" // East Caribbean Dollar
	XOF      = "xof" // West African Cfa Franc*
	XPF      = "xpf" // Cfp Franc*
	YER      = "yer" // Yemeni Rial
	ZAR      = "zar" // South African Rand
	ZMW      = "zmw" // Zambian Kwacha
)

var Types = []Type{
	USD,
	AUD,
	CAD,
	EUR,
	GBP,
	HKD,
	JPY,
	NZD,
	SGD,
	AED,
	AFN,
	ALL,
	AMD,
	ANG,
	AOA,
	ARS,
	AWG,
	AZN,
	BAM,
	BBD,
	BDT,
	BGN,
	BIF,
	BMD,
	BND,
	BOB,
	BRL,
	BSD,
	BWP,
	BZD,
	CDF,
	CHF,
	CLP,
	CNY,
	COP,
	CRC,
	CVE,
	CZK,
	DJF,
	DKK,
	DOP,
	DZD,
	EEK,
	EGP,
	ETB,
	FJD,
	FKP,
	GEL,
	GIP,
	GMD,
	GNF,
	GTQ,
	GYD,
	HNL,
	HRK,
	HTG,
	HUF,
	IDR,
	ILS,
	INR,
	ISK,
	JMD,
	KES,
	KGS,
	KHR,
	KMF,
	KRW,
	KYD,
	KZT,
	LAK,
	LBP,
	LKR,
	LRD,
	LSL,
	LTL,
	LVL,
	MAD,
	MDL,
	MGA,
	MKD,
	MNT,
	MOP,
	MRO,
	MUR,
	MVR,
	MWK,
	MXN,
	MYR,
	MZN,
	NAD,
	NGN,
	NIO,
	NOK,
	NPR,
	PAB,
	PEN,
	PGK,
	PHP,
	PKR,
	PLN,
	PYG,
	QAR,
	RON,
	RSD,
	RUB,
	RWF,
	SAR,
	SBD,
	SCR,
	SEK,
	SHP,
	SLL,
	SOS,
	SRD,
	STD,
	SVC,
	SZL,
	THB,
	TJS,
	TOP,
	TRY,
	TTD,
	TWD,
	TZS,
	UAH,
	UGX,
	UYU,
	UZS,
	VND,
	VUV,
	UWT,
	XAF,
	XCD,
	XOF,
	XPF,
	YER,
	ZAR,
	ZMW,
}
