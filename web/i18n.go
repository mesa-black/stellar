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
		"meta.description":   "Every website key has a sky. Stellar draws a site's public key as a constellation you can recognise at a glance.",
		"tagline":            "Every key has its stargate.",
		"form.label":         "Website, or a <code>SHA256:</code> fingerprint",
		"form.submit":        "Show its sky",
		"result.remember":    "Remember the new sky",
		"fact.keyof":         "Key of",
		"fact.kind":          "Kind",
		"fact.fingerprint":   "Fingerprint",
		"fact.certificate":   "Certificate",
		"fact.connection":    "Connection",
		"fact.stardate":      "Stardate",
		"cert.until":         ", valid until %s",
		"cert.issuer":        ", issued by %s",
		"conn.exchange":      ", key exchange %s",
		"kind.certificate":   "X.509 certificate",
		"kind.publickey":     "public key",
		"kind.fingerprint":   "fingerprint",
		"pq.yes":             "<strong>Post-quantum key exchange: yes.</strong> What travels to this site today stays secret even if it is recorded now and a quantum computer is pointed at it later.",
		"pq.no":              "<strong>Post-quantum key exchange: no.</strong> What travels to this site today can be recorded now and read later by a quantum computer.",
		"pq.key":             "The key drawn here is another matter: it is a classical one, as in every certificate issued today, and it only says who answers now.",
		"dane.yes":           "<strong>Announced by the owner: yes.</strong> The domain's DNS publishes this very key (a DANE record), so this sky is the one its owner says to expect.",
		"dane.other":         "<strong>Announced by the owner: another key.</strong> The domain's DNS publishes %s, and this key is not among them. Either the record is out of date, or someone else is answering.",
		"dane.signed":        "<strong>And the record is signed.</strong> The resolver this server asks checked the domain's DNSSEC signature; Stellar relies on it and does not check a second time.",
		"dane.trust":         "As far as the DNS can be trusted: the record is not signed (DNSSEC), or the resolver this server asks did not check it.",
		"dane.unread":        "This domain publishes DANE records of a form Stellar does not read.",
		"dane.owner.summary": "For the owner: announce this sky",
		"dane.owner.text":    "A domain can say in its DNS which key to expect from it. This record announces the key drawn here; anyone, Stellar included, can then tell that this sky is the right one. It has to follow the key: publish the new one before the site changes its key.",
		"logs.title":         "In the public logs",
		"logs.offer":         "Every certificate issued for a name is written in public logs. Stellar can read them and draw the keys that hold a valid certificate for this site today. To do so it sends this site's name to Cert Spotter (SSLMate), the service that reads the logs.",
		"logs.ask":           "Look in the public logs",
		"logs.shown":         "The keys that hold a certificate valid today for this name, most recent first. More than one is ordinary around a renewal. A sky nobody recognises is a certificate somebody obtained for this name.",
		"logs.absent":        "The key this site showed is not among them.",
		"logs.current":       "shown now",
		"logs.span":          "%s to %s",
		"logs.source":        "Read through Cert Spotter (SSLMate), which answers with the certificates that have not expired: this is the present, not a history.",
		"logs.empty":         "The public logs hold no valid certificate for this name.",
		"logs.busy":          "The service that reads the logs accepts no more questions for now. Try again in an hour.",
		"logs.failed":        "The public logs could not be read just now.",
		"share.label":        "Share this sky",
		"share.button":       "Send on Threema",
		"embed.summary":      "Embed this sky",
		"embed.text":         "The image follows the key: if the site changes its key, the sky changes.",
		"self.caption":       "The sky of this very site: <strong>%s</strong>, the key %s answers with.",
		"self.label":         "The sky of %s",
		"about.what":         "A website proves who it is with a public key. Its fingerprint is 43 characters nobody reads. Stellar draws those characters as a constellation with a name, so a change is something you can see.",
		"about.names":        "The names are invented, and so are the drawings: none is a real constellation. The sky has only 88 of those, far too few for every key to have its own, and a name anybody could pick would prove nothing. So each name is built from the key itself, with a Latin sound, to be said aloud: \"on my side your site is %s; and on yours?\"",
		"about.step1":        "Stellar asks the site for its certificate and hashes the public key inside.",
		"about.step2":        "The hash decides every star: where it sits, how bright it is, what it joins.",
		"about.step3":        "Your browser remembers the sky. Come back later and it tells you whether it is still the same.",
		"about.which":        "A site name shows the sky of the key that site answers with. A document signed with other keys has another sky: to see that one, paste the <code>SHA256:</code> line the document prints.",
		"paste.summary":      "Or paste a public key",
		"paste.label":        "OpenSSH public key, PEM public key, PEM certificate, or SHA256 fingerprint",
		"footer.text":        "Stellar is a visual fingerprint, not a cipher. A sky is a quick way to notice a change; when it matters, compare the full fingerprint.",
		"gate.label":         "Gate",
		"gate.origin":        "origin",
		"gate.text":          "Six signs name this key, in this order. They come from a catalogue of 64 signs shared by every site: they do not look like the sky above, they lead to it, as coordinates do. The seventh, under the chevron, is the sky of this site, which read the key. Same six signs in the same order: same key, as far as an aid can tell. The fingerprint decides.",
		"gate.text.alone":    "Six signs name this key, in this order. They come from a catalogue of 64 signs shared by every site: they do not look like the sky above, they lead to it, as coordinates do. Same six signs in the same order: same key, as far as an aid can tell. The fingerprint decides.",
		"download.label":     "Download the sky map",
		"chart.label":        "Map",
		"chart.download":     "Download this map",
		"chart.with.label":   "Place another site on this map",
		"chart.with.submit":  "Compare",
		"chart.with.far":     "<strong>%s</strong>, whose sky is %s, stands %s radius from here: the green point.",
		"chart.with.same":    "<strong>%s</strong> serves the same key: the two places are one.",
		"chart.arrival":      "arrival",
		"chart.text":         "The small points are the 64 signs of the catalogue: each has a fixed place on a sphere, the same for every key. The six signs of the gate are lit and numbered; taken two by two (1–2, 3–4, 5–6), they draw three lines. Three lines in space seldom cross: the point of arrival is the one nearest the three of them, and a dotted stroke ties it to the nearest point of each line. It stands in the middle of those three points. The point of origin stands outside the sphere: it is that of this site, found the same way from its own key, then pushed out in the same direction. Drag to turn the map.",
		"chart.text.alone":   "The small points are the 64 signs of the catalogue: each has a fixed place on a sphere, the same for every key. The six signs of the gate are lit and numbered; taken two by two (1–2, 3–4, 5–6), they draw three lines. Three lines in space seldom cross: the point of arrival is the one nearest the three of them, and a dotted stroke ties it to the nearest point of each line. It stands in the middle of those three points. Drag to turn the map.",
		"sound.first":        "♪ Sound at the first click · cut",
		"sound.cut":          "♪ Cut the sound",
		"sound.back":         "♪ Sound back on",
		"error.host":         "Enter a public host name, such as example.com.",
		"error.private":      "That name does not lead to a public address.",
		"error.busy":         "Stellar is busy. Try again in a moment.",
		"error.unknown":      "No such host.",
		"error.cert":         "The host answered, but its certificate could not be verified: it is expired, self-signed, or issued for another name. Stellar only draws keys it can verify.",
		"error.timeout":      "The host took too long to answer on port 443.",
		"error.reach":        "Could not reach that host over TLS on port 443.",
		"error.key":          "That is not a public key Stellar can read.",
		"error.print":        "That is not a fingerprint Stellar can read: it takes SHA256: followed by 43 characters.",
		"seen.new":           "First time this browser sees this sky. It will remember it.",
		"seen.same":          "Same sky as on your earlier visits. First seen on {date}.",
		"seen.changed":       "This sky has changed. On {date} this site was {name}. A site gets a new sky when it replaces its key, which is routine at certificate renewal, and also when someone else answers in its place.",
		"seen.accepted":      "Remembered. {name} is now the sky this browser expects.",
		"share.host":         "Stellar · %s\n\nThe sky of its key is %s.\n",
		"share.key":          "Stellar · %s\n\n",
	},
	"fr": {
		"meta.description":   "Chaque clé de site web a un ciel. Stellar dessine la clé publique d'un site sous la forme d'une constellation qu'on reconnaît d'un coup d'œil.",
		"tagline":            "À chaque clé sa porte des étoiles.",
		"form.label":         "Site web, ou empreinte <code>SHA256:</code>",
		"form.submit":        "Voir son ciel",
		"result.remember":    "Retenir le nouveau ciel",
		"fact.keyof":         "Clé de",
		"fact.kind":          "Nature",
		"fact.fingerprint":   "Empreinte",
		"fact.certificate":   "Certificat",
		"fact.connection":    "Connexion",
		"fact.stardate":      "Date stellaire",
		"cert.until":         ", valide jusqu'au %s",
		"cert.issuer":        ", émis par %s",
		"conn.exchange":      ", échange de clés %s",
		"kind.certificate":   "certificat X.509",
		"kind.publickey":     "clé publique",
		"kind.fingerprint":   "empreinte",
		"pq.yes":             "<strong>Échange de clés post-quantique : oui.</strong> Ce qui circule vers ce site aujourd'hui reste secret, même enregistré maintenant et soumis plus tard à un ordinateur quantique.",
		"pq.no":              "<strong>Échange de clés post-quantique : non.</strong> Ce qui circule vers ce site aujourd'hui peut être enregistré maintenant et lu plus tard par un ordinateur quantique.",
		"pq.key":             "La clé dessinée ici est une autre affaire : elle est classique, comme dans tout certificat émis aujourd'hui, et dit seulement qui répond maintenant.",
		"dane.yes":           "<strong>Annoncé par le propriétaire\u00a0: oui.</strong> Le DNS du domaine publie cette clé même (un enregistrement DANE)\u00a0: ce ciel est celui que son propriétaire dit d'attendre.",
		"dane.other":         "<strong>Annoncé par le propriétaire\u00a0: une autre clé.</strong> Le DNS du domaine publie %s, et cette clé n'en fait pas partie. Soit l'enregistrement est périmé, soit quelqu'un d'autre répond.",
		"dane.signed":        "<strong>Et l'enregistrement est signé.</strong> Le résolveur que ce serveur interroge a vérifié la signature DNSSEC du domaine\u00a0; Stellar s'y fie et ne vérifie pas une seconde fois.",
		"dane.trust":         "Dans la mesure où le DNS est fiable\u00a0: l'enregistrement n'est pas signé (DNSSEC), ou le résolveur que ce serveur interroge ne l'a pas vérifié.",
		"dane.unread":        "Ce domaine publie des enregistrements DANE d'une forme que Stellar ne lit pas.",
		"dane.owner.summary": "Pour le propriétaire\u00a0: annoncer ce ciel",
		"dane.owner.text":    "Un domaine peut dire dans son DNS quelle clé attendre de lui. Cet enregistrement annonce la clé dessinée ici\u00a0; chacun, Stellar compris, peut alors dire que ce ciel est le bon. Il doit suivre la clé\u00a0: publiez le nouveau avant que le site ne change de clé.",
		"logs.title":         "Dans les journaux publics",
		"logs.offer":         "Tout certificat émis pour un nom est inscrit dans des journaux publics. Stellar peut les lire et dessiner les clés qui détiennent aujourd'hui un certificat valide pour ce site. Pour cela, il envoie le nom de ce site à Cert Spotter (SSLMate), le service qui lit les journaux.",
		"logs.ask":           "Regarder dans les journaux publics",
		"logs.shown":         "Les clés qui détiennent un certificat valide aujourd'hui pour ce nom, de la plus récente à la plus ancienne. En voir plusieurs est courant autour d'un renouvellement. Un ciel que personne ne reconnaît, c'est un certificat que quelqu'un a obtenu pour ce nom.",
		"logs.absent":        "La clé que ce site a montrée n'en fait pas partie.",
		"logs.current":       "montrée maintenant",
		"logs.span":          "du %s au %s",
		"logs.source":        "Lu par l'intermédiaire de Cert Spotter (SSLMate), qui répond avec les certificats non expirés\u00a0: c'est le présent, pas un historique.",
		"logs.empty":         "Les journaux publics ne contiennent aucun certificat valide pour ce nom.",
		"logs.busy":          "Le service qui lit les journaux n'accepte plus de questions pour l'instant. Réessayez dans une heure.",
		"logs.failed":        "Les journaux publics n'ont pas pu être lus à l'instant.",
		"share.label":        "Partager ce ciel",
		"share.button":       "Envoyer sur Threema",
		"embed.summary":      "Intégrer ce ciel",
		"embed.text":         "L'image suit la clé : si le site change de clé, le ciel change.",
		"self.caption":       "Le ciel de ce site même : <strong>%s</strong>, la clé avec laquelle %s répond.",
		"self.label":         "Le ciel de %s",
		"about.what":         "Un site web prouve qui il est avec une clé publique. Son empreinte fait 43 caractères que personne ne lit. Stellar dessine ces caractères sous la forme d'une constellation qui porte un nom : un changement devient quelque chose qui se voit.",
		"about.names":        "Les noms sont inventés, les dessins aussi : aucun n'est une vraie constellation. Le ciel n'en compte que 88, bien trop peu pour que chaque clé ait la sienne, et un nom que n'importe qui pourrait choisir ne prouverait rien. Chaque nom est donc tiré de la clé elle-même, avec une sonorité latine, pour être dit à voix haute : « chez moi, votre site s'appelle %s ; et chez vous ? »",
		"about.step1":        "Stellar demande au site son certificat et calcule l'empreinte de la clé publique qu'il contient.",
		"about.step2":        "L'empreinte décide de chaque étoile : sa place, son éclat, ce à quoi elle se relie.",
		"about.step3":        "Votre navigateur retient le ciel. Revenez plus tard : il vous dira si c'est toujours le même.",
		"about.which":        "Un nom de site montre le ciel de la clé avec laquelle ce site répond. Un document signé avec d'autres clés a un autre ciel : pour le voir, collez la ligne <code>SHA256:</code> que le document imprime.",
		"paste.summary":      "Ou coller une clé publique",
		"paste.label":        "Clé publique OpenSSH, clé publique PEM, certificat PEM ou empreinte SHA256",
		"footer.text":        "Stellar est une empreinte visuelle, pas un chiffrement. Un ciel est un moyen rapide de remarquer un changement ; quand l'enjeu est réel, comparez l'empreinte complète.",
		"gate.label":         "Porte",
		"gate.origin":        "origine",
		"gate.text":          "Six signes désignent cette clé, dans cet ordre. Ils viennent d'un catalogue de 64 signes commun à tous les sites : ils ne ressemblent pas au ciel ci-dessus, ils y mènent, comme des coordonnées. Le septième, sous le chevron, est le ciel de ce site, qui a lu la clé. Mêmes six signes dans le même ordre : même clé, pour autant qu'une aide puisse le dire. C'est l'empreinte qui tranche.",
		"gate.text.alone":    "Six signes désignent cette clé, dans cet ordre. Ils viennent d'un catalogue de 64 signes commun à tous les sites : ils ne ressemblent pas au ciel ci-dessus, ils y mènent, comme des coordonnées. Mêmes six signes dans le même ordre : même clé, pour autant qu'une aide puisse le dire. C'est l'empreinte qui tranche.",
		"download.label":     "Télécharger la carte du ciel",
		"chart.label":        "Carte",
		"chart.download":     "Télécharger cette carte",
		"chart.with.label":   "Placer un autre site sur cette carte",
		"chart.with.submit":  "Comparer",
		"chart.with.far":     "<strong>%s</strong>, dont le ciel est %s, se trouve à %s rayon d'ici : le point vert.",
		"chart.with.same":    "<strong>%s</strong> sert la même clé : les deux points n'en font qu'un.",
		"chart.arrival":      "arrivée",
		"chart.text":         "Les petits points sont les 64 signes du catalogue : chacun a une place fixe sur une sphère, la même pour toutes les clés. Les six signes de la porte sont allumés et numérotés ; pris deux par deux (1–2, 3–4, 5–6), ils tracent trois lignes. Trois lignes dans l'espace ne se croisent presque jamais : le point d'arrivée est celui qui passe au plus près des trois, et un trait pointillé le relie au point le plus proche de chaque ligne. Il est au milieu de ces trois points. Le point d'origine est hors de la sphère : c'est celui de ce site, trouvé de la même façon à partir de sa propre clé, puis repoussé au-dehors dans la même direction. Faites glisser pour tourner la carte.",
		"chart.text.alone":   "Les petits points sont les 64 signes du catalogue : chacun a une place fixe sur une sphère, la même pour toutes les clés. Les six signes de la porte sont allumés et numérotés ; pris deux par deux (1–2, 3–4, 5–6), ils tracent trois lignes. Trois lignes dans l'espace ne se croisent presque jamais : le point d'arrivée est celui qui passe au plus près des trois, et un trait pointillé le relie au point le plus proche de chaque ligne. Il est au milieu de ces trois points. Faites glisser pour tourner la carte.",
		"sound.first":        "♪ Son au premier clic · couper",
		"sound.cut":          "♪ Couper le son",
		"sound.back":         "♪ Remettre le son",
		"error.host":         "Saisissez un nom de site public, par exemple example.com.",
		"error.private":      "Ce nom ne mène pas à une adresse publique.",
		"error.busy":         "Stellar est occupé. Réessayez dans un instant.",
		"error.unknown":      "Ce site n'existe pas.",
		"error.cert":         "Le site a répondu, mais son certificat n'a pas pu être vérifié : il est expiré, auto-signé ou émis pour un autre nom. Stellar ne dessine que les clés qu'il peut vérifier.",
		"error.timeout":      "Le site a mis trop de temps à répondre sur le port 443.",
		"error.reach":        "Impossible de joindre ce site en TLS sur le port 443.",
		"error.key":          "Ce n'est pas une clé publique que Stellar sait lire.",
		"error.print":        "Ce n'est pas une empreinte que Stellar sait lire : il faut SHA256: suivi de 43 caractères.",
		"seen.new":           "Première fois que ce navigateur voit ce ciel. Il le retiendra.",
		"seen.same":          "Même ciel que lors de vos visites précédentes. Vu pour la première fois le {date}.",
		"seen.changed":       "Ce ciel a changé. Le {date}, ce site était {name}. Un site change de ciel quand il remplace sa clé, ce qui est courant au renouvellement du certificat, et aussi quand quelqu'un d'autre répond à sa place.",
		"seen.accepted":      "C'est noté. {name} est désormais le ciel que ce navigateur attend.",
		"share.host":         "Stellar · %s\n\nLe ciel de sa clé s'appelle %s.\n",
		"share.key":          "Stellar · %s\n\n",
	},
	"es": {
		"meta.description":   "Cada clave de un sitio web tiene un cielo. Stellar dibuja la clave pública de un sitio como una constelación que se reconoce de un vistazo.",
		"tagline":            "Cada clave tiene su stargate.",
		"form.label":         "Sitio web, o huella <code>SHA256:</code>",
		"form.submit":        "Ver su cielo",
		"result.remember":    "Recordar el nuevo cielo",
		"fact.keyof":         "Clave de",
		"fact.kind":          "Tipo",
		"fact.fingerprint":   "Huella",
		"fact.certificate":   "Certificado",
		"fact.connection":    "Conexión",
		"fact.stardate":      "Fecha estelar",
		"cert.until":         ", válido hasta el %s",
		"cert.issuer":        ", emitido por %s",
		"conn.exchange":      ", intercambio de claves %s",
		"kind.certificate":   "certificado X.509",
		"kind.publickey":     "clave pública",
		"kind.fingerprint":   "huella",
		"pq.yes":             "<strong>Intercambio de claves poscuántico: sí.</strong> Lo que viaja hoy hacia este sitio sigue siendo secreto, aunque se grabe ahora y se someta más tarde a un ordenador cuántico.",
		"pq.no":              "<strong>Intercambio de claves poscuántico: no.</strong> Lo que viaja hoy hacia este sitio puede grabarse ahora y leerse más tarde con un ordenador cuántico.",
		"pq.key":             "La clave dibujada aquí es otro asunto: es clásica, como en todo certificado emitido hoy, y solo dice quién responde ahora.",
		"dane.yes":           "<strong>Anunciado por el propietario: sí.</strong> El DNS del dominio publica esta misma clave (un registro DANE), así que este cielo es el que su propietario dice que hay que esperar.",
		"dane.other":         "<strong>Anunciado por el propietario: otra clave.</strong> El DNS del dominio publica %s, y esta clave no está entre ellas. O el registro está desactualizado, o responde otro.",
		"dane.signed":        "<strong>Y el registro está firmado.</strong> El resolutor que este servidor consulta ha verificado la firma DNSSEC del dominio; Stellar se fía de él y no la verifica una segunda vez.",
		"dane.trust":         "En la medida en que el DNS sea fiable: el registro no está firmado (DNSSEC), o el resolutor que este servidor consulta no lo ha verificado.",
		"dane.unread":        "Este dominio publica registros DANE de una forma que Stellar no lee.",
		"dane.owner.summary": "Para el propietario: anunciar este cielo",
		"dane.owner.text":    "Un dominio puede decir en su DNS qué clave esperar de él. Este registro anuncia la clave dibujada aquí; cualquiera, Stellar incluido, puede entonces decir que este cielo es el correcto. Debe seguir a la clave: publique el nuevo antes de que el sitio cambie de clave.",
		"logs.title":         "En los registros públicos",
		"logs.offer":         "Todo certificado emitido para un nombre queda inscrito en registros públicos. Stellar puede leerlos y dibujar las claves que tienen hoy un certificado válido para este sitio. Para ello envía el nombre de este sitio a Cert Spotter (SSLMate), el servicio que lee los registros.",
		"logs.ask":           "Mirar en los registros públicos",
		"logs.shown":         "Las claves que tienen un certificado válido hoy para este nombre, de la más reciente a la más antigua. Ver varias es habitual cerca de una renovación. Un cielo que nadie reconoce es un certificado que alguien obtuvo para este nombre.",
		"logs.absent":        "La clave que este sitio mostró no está entre ellas.",
		"logs.current":       "mostrada ahora",
		"logs.span":          "del %s al %s",
		"logs.source":        "Leído a través de Cert Spotter (SSLMate), que responde con los certificados no caducados: es el presente, no un historial.",
		"logs.empty":         "Los registros públicos no contienen ningún certificado válido para este nombre.",
		"logs.busy":          "El servicio que lee los registros no acepta más preguntas por ahora. Vuelva a intentarlo en una hora.",
		"logs.failed":        "Los registros públicos no han podido leerse en este momento.",
		"share.label":        "Compartir este cielo",
		"share.button":       "Enviar por Threema",
		"embed.summary":      "Insertar este cielo",
		"embed.text":         "La imagen sigue a la clave: si el sitio cambia de clave, el cielo cambia.",
		"self.caption":       "El cielo de este mismo sitio: <strong>%s</strong>, la clave con la que responde %s.",
		"self.label":         "El cielo de %s",
		"about.what":         "Un sitio web demuestra quién es con una clave pública. Su huella son 43 caracteres que nadie lee. Stellar dibuja esos caracteres como una constelación con nombre, para que un cambio sea algo que se ve.",
		"about.names":        "Los nombres son inventados, y los dibujos también: ninguno es una constelación real. El cielo solo tiene 88, demasiado pocas para que cada clave tenga la suya, y un nombre que cualquiera pudiera elegir no probaría nada. Por eso cada nombre se construye a partir de la propia clave, con sonoridad latina, para decirlo en voz alta: «en mi lado su sitio se llama %s; ¿y en el suyo?»",
		"about.step1":        "Stellar pide al sitio su certificado y calcula la huella de la clave pública que contiene.",
		"about.step2":        "La huella decide cada estrella: dónde está, cuánto brilla, con qué se une.",
		"about.step3":        "Su navegador recuerda el cielo. Vuelva más tarde y le dirá si sigue siendo el mismo.",
		"about.which":        "Un nombre de sitio muestra el cielo de la clave con la que ese sitio responde. Un documento firmado con otras claves tiene otro cielo: para verlo, pegue la línea <code>SHA256:</code> que el documento imprime.",
		"paste.summary":      "O pegar una clave pública",
		"paste.label":        "Clave pública OpenSSH, clave pública PEM, certificado PEM o huella SHA256",
		"footer.text":        "Stellar es una huella visual, no un cifrado. Un cielo es una forma rápida de advertir un cambio; cuando importa, compare la huella completa.",
		"gate.label":         "Puerta",
		"gate.origin":        "origen",
		"gate.text":          "Seis signos designan esta clave, en este orden. Vienen de un catálogo de 64 signos común a todos los sitios: no se parecen al cielo de arriba, llevan a él, como unas coordenadas. El séptimo, bajo el cheurón, es el cielo de este sitio, que leyó la clave. Los mismos seis signos en el mismo orden: la misma clave, hasta donde una ayuda puede decirlo. La huella decide.",
		"gate.text.alone":    "Seis signos designan esta clave, en este orden. Vienen de un catálogo de 64 signos común a todos los sitios: no se parecen al cielo de arriba, llevan a él, como unas coordenadas. Los mismos seis signos en el mismo orden: la misma clave, hasta donde una ayuda puede decirlo. La huella decide.",
		"download.label":     "Descargar el mapa del cielo",
		"chart.label":        "Mapa",
		"chart.download":     "Descargar este mapa",
		"chart.with.label":   "Situar otro sitio en este mapa",
		"chart.with.submit":  "Comparar",
		"chart.with.far":     "<strong>%s</strong>, cuyo cielo es %s, está a %s radio de aquí: el punto verde.",
		"chart.with.same":    "<strong>%s</strong> sirve la misma clave: los dos puntos son uno solo.",
		"chart.arrival":      "llegada",
		"chart.text":         "Los puntos pequeños son los 64 signos del catálogo: cada uno tiene un lugar fijo en una esfera, el mismo para todas las claves. Los seis signos de la puerta están encendidos y numerados; tomados de dos en dos (1–2, 3–4, 5–6), trazan tres líneas. Tres líneas en el espacio casi nunca se cruzan: el punto de llegada es el que queda más cerca de las tres, y un trazo punteado lo une al punto más cercano de cada línea. Está en medio de esos tres puntos. El punto de origen queda fuera de la esfera: es el de este sitio, hallado del mismo modo a partir de su propia clave, y luego llevado hacia fuera en la misma dirección. Arrastre para girar el mapa.",
		"chart.text.alone":   "Los puntos pequeños son los 64 signos del catálogo: cada uno tiene un lugar fijo en una esfera, el mismo para todas las claves. Los seis signos de la puerta están encendidos y numerados; tomados de dos en dos (1–2, 3–4, 5–6), trazan tres líneas. Tres líneas en el espacio casi nunca se cruzan: el punto de llegada es el que queda más cerca de las tres, y un trazo punteado lo une al punto más cercano de cada línea. Está en medio de esos tres puntos. Arrastre para girar el mapa.",
		"sound.first":        "♪ Sonido al primer clic · cortar",
		"sound.cut":          "♪ Cortar el sonido",
		"sound.back":         "♪ Volver a poner el sonido",
		"error.host":         "Escriba un nombre de sitio público, por ejemplo example.com.",
		"error.private":      "Ese nombre no lleva a una dirección pública.",
		"error.busy":         "Stellar está ocupado. Vuelva a intentarlo en un momento.",
		"error.unknown":      "Ese sitio no existe.",
		"error.cert":         "El sitio respondió, pero su certificado no pudo verificarse: está caducado, autofirmado o emitido para otro nombre. Stellar solo dibuja las claves que puede verificar.",
		"error.timeout":      "El sitio tardó demasiado en responder en el puerto 443.",
		"error.reach":        "No se pudo alcanzar ese sitio por TLS en el puerto 443.",
		"error.key":          "Eso no es una clave pública que Stellar sepa leer.",
		"error.print":        "Eso no es una huella que Stellar sepa leer: hace falta SHA256: seguido de 43 caracteres.",
		"seen.new":           "Primera vez que este navegador ve este cielo. Lo recordará.",
		"seen.same":          "El mismo cielo que en sus visitas anteriores. Visto por primera vez el {date}.",
		"seen.changed":       "Este cielo ha cambiado. El {date}, este sitio era {name}. Un sitio cambia de cielo cuando sustituye su clave, algo habitual al renovar el certificado, y también cuando otro responde en su lugar.",
		"seen.accepted":      "Anotado. {name} es ahora el cielo que este navegador espera.",
		"share.host":         "Stellar · %s\n\nEl cielo de su clave se llama %s.\n",
		"share.key":          "Stellar · %s\n\n",
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
