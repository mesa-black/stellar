package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

// The site speaks three languages. English governs: a key missing from
// another language falls back to it, and a test refuses a catalogue whose
// keys or placeholders differ from the English one.
var languages = []string{"en", "fr", "es"}

var languageNames = map[string]string{"en": "English", "fr": "Français", "es": "Español"}

// Sentences may carry markup (<strong>, <code>): they are constants of this
// file, never a visitor's words. What a visitor or a certificate supplies is
// passed as an argument and escaped (see view.T).
var messages = map[string]map[string]string{
	"en": {
		"meta.description": "Every website key has a sky. Stellar draws a site's public key as a constellation you can recognise at a glance.",
		"tagline":          "Every key has a sky.",
		"form.label":       "Website, or a <code>SHA256:</code> fingerprint",
		"form.submit":      "Show its sky",
		"result.remember":  "Remember the new sky",
		"fact.keyof":       "Key of",
		"fact.kind":        "Kind",
		"fact.fingerprint": "Fingerprint",
		"fact.certificate": "Certificate",
		"fact.connection":  "Connection",
		"cert.until":       ", valid until %s",
		"cert.issuer":      ", issued by %s",
		"conn.exchange":    ", key exchange %s",
		"kind.certificate": "X.509 certificate",
		"kind.publickey":   "public key",
		"kind.fingerprint": "fingerprint",
		"pq.yes":           "<strong>Post-quantum key exchange: yes.</strong> What travels to this site today stays secret even if it is recorded now and a quantum computer is pointed at it later.",
		"pq.no":            "<strong>Post-quantum key exchange: no.</strong> What travels to this site today can be recorded now and read later by a quantum computer.",
		"pq.key":           "The key drawn here is another matter: it is a classical one, as in every certificate issued today, and it only says who answers now.",
		"share.label":      "Share this sky",
		"share.button":     "Send on Threema",
		"embed.summary":    "Embed this sky",
		"embed.text":       "The image follows the key: if the site changes its key, the sky changes.",
		"self.caption":     "The sky of this very site: <strong>%s</strong>, the key %s answers with.",
		"self.label":       "The sky of %s",
		"about.what":       "A website proves who it is with a public key. Its fingerprint is 43 characters nobody reads. Stellar draws those characters as a constellation with a name, so a change is something you can see.",
		"about.names":      "The names are invented, and so are the drawings: none is a real constellation. The sky has only 88 of those, far too few for every key to have its own, and a name anybody could pick would prove nothing. So each name is built from the key itself, with a Latin sound, to be said aloud: \"on my side your site is %s; and on yours?\"",
		"about.step1":      "Stellar asks the site for its certificate and hashes the public key inside.",
		"about.step2":      "The hash decides every star: where it sits, how bright it is, what it joins.",
		"about.step3":      "Your browser remembers the sky. Come back later and it tells you whether it is still the same.",
		"about.which":      "A site name shows the sky of the key that site answers with. A document signed with other keys has another sky: to see that one, paste the <code>SHA256:</code> line the document prints.",
		"paste.summary":    "Or paste a public key",
		"paste.label":      "OpenSSH public key, PEM public key, PEM certificate, or SHA256 fingerprint",
		"footer.text":      "Stellar is a visual fingerprint, not a cipher. A sky is a quick way to notice a change; when it matters, compare the full fingerprint.",
		"error.host":       "Enter a public host name, such as example.com.",
		"error.private":    "That name does not lead to a public address.",
		"error.busy":       "Stellar is busy. Try again in a moment.",
		"error.unknown":    "No such host.",
		"error.cert":       "The host answered, but its certificate could not be verified: it is expired, self-signed, or issued for another name. Stellar only draws keys it can verify.",
		"error.timeout":    "The host took too long to answer on port 443.",
		"error.reach":      "Could not reach that host over TLS on port 443.",
		"error.key":        "That is not a public key Stellar can read.",
		"error.print":      "That is not a fingerprint Stellar can read: it takes SHA256: followed by 43 characters.",
		"seen.new":         "First time this browser sees this sky. It will remember it.",
		"seen.same":        "Same sky as on your earlier visits. First seen on {date}.",
		"seen.changed":     "This sky has changed. On {date} this site was {name}. A site gets a new sky when it replaces its key, which is routine at certificate renewal, and also when someone else answers in its place.",
		"seen.accepted":    "Remembered. {name} is now the sky this browser expects.",
		"share.host":       "Stellar · %s\n\nThe sky of its key is %s.\n",
		"share.key":        "Stellar · %s\n\n",
	},
	"fr": {
		"meta.description": "Chaque clé de site web a un ciel. Stellar dessine la clé publique d'un site sous la forme d'une constellation qu'on reconnaît d'un coup d'œil.",
		"tagline":          "À chaque clé son ciel.",
		"form.label":       "Site web, ou empreinte <code>SHA256:</code>",
		"form.submit":      "Voir son ciel",
		"result.remember":  "Retenir le nouveau ciel",
		"fact.keyof":       "Clé de",
		"fact.kind":        "Nature",
		"fact.fingerprint": "Empreinte",
		"fact.certificate": "Certificat",
		"fact.connection":  "Connexion",
		"cert.until":       ", valide jusqu'au %s",
		"cert.issuer":      ", émis par %s",
		"conn.exchange":    ", échange de clés %s",
		"kind.certificate": "certificat X.509",
		"kind.publickey":   "clé publique",
		"kind.fingerprint": "empreinte",
		"pq.yes":           "<strong>Échange de clés post-quantique : oui.</strong> Ce qui circule vers ce site aujourd'hui reste secret, même enregistré maintenant et soumis plus tard à un ordinateur quantique.",
		"pq.no":            "<strong>Échange de clés post-quantique : non.</strong> Ce qui circule vers ce site aujourd'hui peut être enregistré maintenant et lu plus tard par un ordinateur quantique.",
		"pq.key":           "La clé dessinée ici est une autre affaire : elle est classique, comme dans tout certificat émis aujourd'hui, et dit seulement qui répond maintenant.",
		"share.label":      "Partager ce ciel",
		"share.button":     "Envoyer sur Threema",
		"embed.summary":    "Intégrer ce ciel",
		"embed.text":       "L'image suit la clé : si le site change de clé, le ciel change.",
		"self.caption":     "Le ciel de ce site même : <strong>%s</strong>, la clé avec laquelle %s répond.",
		"self.label":       "Le ciel de %s",
		"about.what":       "Un site web prouve qui il est avec une clé publique. Son empreinte fait 43 caractères que personne ne lit. Stellar dessine ces caractères sous la forme d'une constellation qui porte un nom : un changement devient quelque chose qui se voit.",
		"about.names":      "Les noms sont inventés, les dessins aussi : aucun n'est une vraie constellation. Le ciel n'en compte que 88, bien trop peu pour que chaque clé ait la sienne, et un nom que n'importe qui pourrait choisir ne prouverait rien. Chaque nom est donc tiré de la clé elle-même, avec une sonorité latine, pour être dit à voix haute : « chez moi, votre site s'appelle %s ; et chez vous ? »",
		"about.step1":      "Stellar demande au site son certificat et calcule l'empreinte de la clé publique qu'il contient.",
		"about.step2":      "L'empreinte décide de chaque étoile : sa place, son éclat, ce à quoi elle se relie.",
		"about.step3":      "Votre navigateur retient le ciel. Revenez plus tard : il vous dira si c'est toujours le même.",
		"about.which":      "Un nom de site montre le ciel de la clé avec laquelle ce site répond. Un document signé avec d'autres clés a un autre ciel : pour le voir, collez la ligne <code>SHA256:</code> que le document imprime.",
		"paste.summary":    "Ou coller une clé publique",
		"paste.label":      "Clé publique OpenSSH, clé publique PEM, certificat PEM ou empreinte SHA256",
		"footer.text":      "Stellar est une empreinte visuelle, pas un chiffrement. Un ciel est un moyen rapide de remarquer un changement ; quand l'enjeu est réel, comparez l'empreinte complète.",
		"error.host":       "Saisissez un nom de site public, par exemple example.com.",
		"error.private":    "Ce nom ne mène pas à une adresse publique.",
		"error.busy":       "Stellar est occupé. Réessayez dans un instant.",
		"error.unknown":    "Ce site n'existe pas.",
		"error.cert":       "Le site a répondu, mais son certificat n'a pas pu être vérifié : il est expiré, auto-signé ou émis pour un autre nom. Stellar ne dessine que les clés qu'il peut vérifier.",
		"error.timeout":    "Le site a mis trop de temps à répondre sur le port 443.",
		"error.reach":      "Impossible de joindre ce site en TLS sur le port 443.",
		"error.key":        "Ce n'est pas une clé publique que Stellar sait lire.",
		"error.print":      "Ce n'est pas une empreinte que Stellar sait lire : il faut SHA256: suivi de 43 caractères.",
		"seen.new":         "Première fois que ce navigateur voit ce ciel. Il le retiendra.",
		"seen.same":        "Même ciel que lors de vos visites précédentes. Vu pour la première fois le {date}.",
		"seen.changed":     "Ce ciel a changé. Le {date}, ce site était {name}. Un site change de ciel quand il remplace sa clé, ce qui est courant au renouvellement du certificat, et aussi quand quelqu'un d'autre répond à sa place.",
		"seen.accepted":    "C'est noté. {name} est désormais le ciel que ce navigateur attend.",
		"share.host":       "Stellar · %s\n\nLe ciel de sa clé s'appelle %s.\n",
		"share.key":        "Stellar · %s\n\n",
	},
	"es": {
		"meta.description": "Cada clave de un sitio web tiene un cielo. Stellar dibuja la clave pública de un sitio como una constelación que se reconoce de un vistazo.",
		"tagline":          "Cada clave tiene su cielo.",
		"form.label":       "Sitio web, o huella <code>SHA256:</code>",
		"form.submit":      "Ver su cielo",
		"result.remember":  "Recordar el nuevo cielo",
		"fact.keyof":       "Clave de",
		"fact.kind":        "Tipo",
		"fact.fingerprint": "Huella",
		"fact.certificate": "Certificado",
		"fact.connection":  "Conexión",
		"cert.until":       ", válido hasta el %s",
		"cert.issuer":      ", emitido por %s",
		"conn.exchange":    ", intercambio de claves %s",
		"kind.certificate": "certificado X.509",
		"kind.publickey":   "clave pública",
		"kind.fingerprint": "huella",
		"pq.yes":           "<strong>Intercambio de claves poscuántico: sí.</strong> Lo que viaja hoy hacia este sitio sigue siendo secreto, aunque se grabe ahora y se someta más tarde a un ordenador cuántico.",
		"pq.no":            "<strong>Intercambio de claves poscuántico: no.</strong> Lo que viaja hoy hacia este sitio puede grabarse ahora y leerse más tarde con un ordenador cuántico.",
		"pq.key":           "La clave dibujada aquí es otro asunto: es clásica, como en todo certificado emitido hoy, y solo dice quién responde ahora.",
		"share.label":      "Compartir este cielo",
		"share.button":     "Enviar por Threema",
		"embed.summary":    "Insertar este cielo",
		"embed.text":       "La imagen sigue a la clave: si el sitio cambia de clave, el cielo cambia.",
		"self.caption":     "El cielo de este mismo sitio: <strong>%s</strong>, la clave con la que responde %s.",
		"self.label":       "El cielo de %s",
		"about.what":       "Un sitio web demuestra quién es con una clave pública. Su huella son 43 caracteres que nadie lee. Stellar dibuja esos caracteres como una constelación con nombre, para que un cambio sea algo que se ve.",
		"about.names":      "Los nombres son inventados, y los dibujos también: ninguno es una constelación real. El cielo solo tiene 88, demasiado pocas para que cada clave tenga la suya, y un nombre que cualquiera pudiera elegir no probaría nada. Por eso cada nombre se construye a partir de la propia clave, con sonoridad latina, para decirlo en voz alta: «en mi lado su sitio se llama %s; ¿y en el suyo?»",
		"about.step1":      "Stellar pide al sitio su certificado y calcula la huella de la clave pública que contiene.",
		"about.step2":      "La huella decide cada estrella: dónde está, cuánto brilla, con qué se une.",
		"about.step3":      "Su navegador recuerda el cielo. Vuelva más tarde y le dirá si sigue siendo el mismo.",
		"about.which":      "Un nombre de sitio muestra el cielo de la clave con la que ese sitio responde. Un documento firmado con otras claves tiene otro cielo: para verlo, pegue la línea <code>SHA256:</code> que el documento imprime.",
		"paste.summary":    "O pegar una clave pública",
		"paste.label":      "Clave pública OpenSSH, clave pública PEM, certificado PEM o huella SHA256",
		"footer.text":      "Stellar es una huella visual, no un cifrado. Un cielo es una forma rápida de advertir un cambio; cuando importa, compare la huella completa.",
		"error.host":       "Escriba un nombre de sitio público, por ejemplo example.com.",
		"error.private":    "Ese nombre no lleva a una dirección pública.",
		"error.busy":       "Stellar está ocupado. Vuelva a intentarlo en un momento.",
		"error.unknown":    "Ese sitio no existe.",
		"error.cert":       "El sitio respondió, pero su certificado no pudo verificarse: está caducado, autofirmado o emitido para otro nombre. Stellar solo dibuja las claves que puede verificar.",
		"error.timeout":    "El sitio tardó demasiado en responder en el puerto 443.",
		"error.reach":      "No se pudo alcanzar ese sitio por TLS en el puerto 443.",
		"error.key":        "Eso no es una clave pública que Stellar sepa leer.",
		"error.print":      "Eso no es una huella que Stellar sepa leer: hace falta SHA256: seguido de 43 caracteres.",
		"seen.new":         "Primera vez que este navegador ve este cielo. Lo recordará.",
		"seen.same":        "El mismo cielo que en sus visitas anteriores. Visto por primera vez el {date}.",
		"seen.changed":     "Este cielo ha cambiado. El {date}, este sitio era {name}. Un sitio cambia de cielo cuando sustituye su clave, algo habitual al renovar el certificado, y también cuando otro responde en su lugar.",
		"seen.accepted":    "Anotado. {name} es ahora el cielo que este navegador espera.",
		"share.host":       "Stellar · %s\n\nEl cielo de su clave se llama %s.\n",
		"share.key":        "Stellar · %s\n\n",
	},
}

var months = map[string][12]string{
	"en": {"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	"fr": {"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
	"es": {"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"},
}

// message returns a sentence in a language, in English when that language
// lacks it, and the key itself when nobody has it: a visible gap.
func message(lang, key string) string {
	if m, ok := messages[lang][key]; ok {
		return m
	}
	if m, ok := messages["en"][key]; ok {
		return m
	}
	return key
}

// longDate writes a day the way the language does: 12 January 2027,
// 12 janvier 2027, 12 de enero de 2027.
func longDate(lang string, t time.Time) string {
	t = t.UTC()
	names, ok := months[lang]
	if !ok {
		lang, names = "en", months["en"]
	}
	month := names[t.Month()-1]
	switch lang {
	case "fr":
		day := fmt.Sprint(t.Day())
		if t.Day() == 1 {
			day = "1er"
		}
		return fmt.Sprintf("%s %s %d", day, month, t.Year())
	case "es":
		return fmt.Sprintf("%d de %s de %d", t.Day(), month, t.Year())
	}
	return fmt.Sprintf("%d %s %d", t.Day(), month, t.Year())
}

// pickLanguage chooses the language of a response. A `lang` parameter is a
// visitor's explicit choice and is carried from page to page; without it the
// browser's Accept-Language decides, and English if it names none of ours.
func pickLanguage(r *http.Request) (lang string, explicit bool) {
	if chosen := r.FormValue("lang"); isLanguage(chosen) {
		return chosen, true
	}
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		primary, _, _ := strings.Cut(strings.ToLower(tag), "-")
		if isLanguage(primary) {
			return primary, false
		}
	}
	return "en", false
}

func isLanguage(code string) bool {
	for _, l := range languages {
		if l == code {
			return true
		}
	}
	return false
}

// T returns a sentence of the page's language as markup. The sentence is a
// constant of this package; its arguments are escaped.
func (v view) T(key string, args ...any) template.HTML {
	msg := message(v.Lang, key)
	if len(args) == 0 {
		return template.HTML(msg)
	}
	escaped := make([]any, len(args))
	for i, a := range args {
		escaped[i] = template.HTMLEscapeString(fmt.Sprint(a))
	}
	return template.HTML(fmt.Sprintf(msg, escaped...))
}

// S returns a sentence as plain text, for an attribute or a message: the
// template escapes it like any other string.
func (v view) S(key string, args ...any) string {
	msg := message(v.Lang, key)
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// KindLabel names the kind of key in the page's language. OpenSSH key types
// (ssh-ed25519…) are names, not words, and stay as they are.
func (v view) KindLabel() string {
	switch v.Kind {
	case "X.509 certificate":
		return v.S("kind.certificate")
	case "public key":
		return v.S("kind.publickey")
	case "fingerprint":
		return v.S("kind.fingerprint")
	}
	return v.Kind
}

// langLink is one entry of the language switcher.
type langLink struct {
	Code, Name, URL string
	Current         bool
}
