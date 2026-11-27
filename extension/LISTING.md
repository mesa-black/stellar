# The listing on addons.mozilla.org

What goes in the form when the extension is submitted, kept here so that the
next version says the same thing. The package is `bin/stellar-extension.zip`,
built by `make extension`.

- **Name**: Stellar
- **Categories**: Privacy & Security
- **Licence**: MIT
- **Homepage**: https://stellar.mesa.black
- **Support site**: https://github.com/mesa-black/stellar/issues
- **This add-on is experimental**: no
- **Requires payment**: no

## Summary (250 characters at most)

**English**
The key of the page in front of you, drawn as a constellation, as your own machine received it. Notice when a site's key changes, and compare it in one click with the key a server elsewhere receives.

**Français**
La clé de la page devant vous, dessinée en constellation, telle que votre machine l'a reçue. Remarquez quand la clé d'un site change, et comparez-la en un clic à celle que reçoit un serveur situé ailleurs.

**Español**
La clave de la página que tiene delante, dibujada como una constelación, tal como la recibió su máquina. Note cuándo cambia la clave de un sitio y compárela con un clic con la que recibe un servidor en otro lugar.

## Description

**English**

When your browser shows a padlock, a site has presented a certificate your machine accepts. Behind a corporate proxy, an antivirus that inspects https or a badly kept wifi, that certificate is not the site's, and the padlock looks the same.

Stellar reads the certificate of the page in each tab from Firefox itself and draws the public key it holds as a constellation with a name: same key, same sky; another key, an unrelated sky.

- The sky of this page: its name, its fingerprint, who issued the certificate and until when, and whether the key exchange was a post-quantum hybrid.
- Whether it changed: the extension remembers the sky each site showed last time, and marks its button when it is not the same.
- Whether a server elsewhere sees the same: one button asks stellar.mesa.black to compare the key your machine received with the one its server receives from the same site. Two different keys is what an interception looks like.

Nothing leaves your machine until you press that button. It then sends the name of the site and the fingerprint of its public key, and nothing else.

A sky is an aid to notice a change, not a proof: when it matters, compare the fingerprint. The code is open, and the mapping from a key to its sky is specified and checked against reference skies.

**Français**

Quand votre navigateur affiche un cadenas, un site a présenté un certificat que votre machine accepte. Derrière un proxy d'entreprise, un antivirus qui inspecte le https ou un wifi mal tenu, ce certificat n'est pas celui du site, et le cadenas a la même allure.

Stellar lit le certificat de la page de chaque onglet dans Firefox lui-même, et dessine la clé publique qu'il contient sous la forme d'une constellation qui porte un nom : même clé, même ciel ; autre clé, ciel sans rapport.

- Le ciel de cette page : son nom, son empreinte, qui a émis le certificat et jusqu'à quand, et si l'échange de clés était hybride post-quantique.
- S'il a changé : l'extension retient le ciel que chaque site montrait la dernière fois, et marque son bouton quand ce n'est plus le même.
- Si un serveur situé ailleurs voit le même : un bouton demande à stellar.mesa.black de comparer la clé reçue par votre machine à celle que son serveur reçoit du même site. Deux clés différentes, c'est à quoi ressemble une interception.

Rien ne quitte votre machine avant ce bouton. Il envoie alors le nom du site et l'empreinte de sa clé publique, et rien d'autre.

Un ciel aide à remarquer un changement, il ne prouve rien : quand l'enjeu est réel, comparez l'empreinte. Le code est ouvert, et le calcul qui mène d'une clé à son ciel est spécifié et vérifié sur des ciels de référence.

**Español**

Cuando su navegador muestra un candado, un sitio ha presentado un certificado que su máquina acepta. Detrás de un proxy de empresa, de un antivirus que inspecciona el https o de un wifi mal cuidado, ese certificado no es el del sitio, y el candado tiene el mismo aspecto.

Stellar lee el certificado de la página de cada pestaña en el propio Firefox y dibuja la clave pública que contiene como una constelación con nombre: la misma clave, el mismo cielo; otra clave, un cielo sin relación.

- El cielo de esta página: su nombre, su huella, quién emitió el certificado y hasta cuándo, y si el intercambio de claves fue híbrido poscuántico.
- Si ha cambiado: la extensión recuerda el cielo que cada sitio mostraba la última vez, y marca su botón cuando ya no es el mismo.
- Si un servidor en otro lugar ve el mismo: un botón pide a stellar.mesa.black que compare la clave que recibió su máquina con la que su servidor recibe del mismo sitio. Dos claves distintas es lo que parece una interceptación.

Nada sale de su máquina antes de ese botón. Entonces envía el nombre del sitio y la huella de su clave pública, y nada más.

Un cielo ayuda a notar un cambio, no prueba nada: cuando importa, compare la huella. El código es abierto, y el cálculo que lleva de una clave a su cielo está especificado y comprobado con cielos de referencia.

## Privacy policy

Stellar collects nothing by itself and has no account, no analytics and no advertising.

It reads, inside your browser, the certificate of the https pages you open, in order to draw the public key each one holds. It keeps, in the extension's own storage in your browser, the fingerprint and the name of the sky each site showed last time, so that it can tell you when it changes. That storage never leaves your browser; removing the extension erases it.

One thing is transmitted, and only when you press "Compare with what the server sees": the name of the site of the current tab and the fingerprint of its public key are sent to https://stellar.mesa.black, which answers whether its own server receives the same key from that site. The page that opens may, if you ask it to, look for that key in the public Certificate Transparency logs; doing so sends the name of the site to Cert Spotter (SSLMate), and the page says so before you ask.

Contact: the issue tracker of https://github.com/mesa-black/stellar.

## Notes to the reviewer

- No build step, no minified or generated code, no remote code: the package is the content of the `extension/` directory of https://github.com/mesa-black/stellar, zipped (`make extension`).
- `webRequest`, `webRequestBlocking` and `https://*/*`: `webRequest.getSecurityInfo` can only be called from a blocking `onHeadersReceived` listener. The listener is registered for `main_frame` only, reads the certificate, and returns nothing: no request is modified, redirected or cancelled, and no page content is read. See `background.js`, some sixty lines.
- `storage`: `storage.session` holds what the page of each tab showed, for the popup; `storage.local` holds the digest and the name each host showed last time.
- Data collection: `browsingActivity` is declared because the button "Compare with what the server sees" opens `https://stellar.mesa.black/verify?host=<host of the tab>&key=<fingerprint>` in a new tab. It is the only transmission, it happens only on that click, and the popup says what is sent just above the button.
- `stellar.js` is a port of the project's mapping (SHAKE256 written out on BigInt, since the browser has none); `node extension/test.js`, in the repository, checks it against 32 reference vectors.
- To test: open any https site, open the popup, press the button.
