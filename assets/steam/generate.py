logo = open('/Users/mirza/Development/rommate/assets/logo.svg').read()

def mark(i, size):
    return (logo.replace('rommate-gradient', 'g%d' % i)
                .replace('rommate-cut', 'c%d' % i)
                .replace('width="128" height="128"', 'width="%d" height="%d"' % (size, size)))

head = '''<!doctype html><meta charset="utf-8">
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Bricolage+Grotesque:opsz,wght@12..96,700;12..96,800&family=IBM+Plex+Mono:wght@400;500&display=swap">
<style>
html,body{margin:0;overflow:hidden}
.rom{background:linear-gradient(90deg,#7B61FF,#FF7AB8);-webkit-background-clip:text;background-clip:text;color:transparent}
.word{font-family:"Bricolage Grotesque",sans-serif;font-weight:800;line-height:.95;letter-spacing:-.035em;color:#ECEAF6}
.tag{font-family:"IBM Plex Mono",monospace;line-height:1.3;color:#A9A6BE}
svg{display:block}
</style>'''

capsule = ('<body style="width:600px;height:900px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:40px;'
    'background:#16161E;background-image:radial-gradient(420px 380px at 50% 34%, rgba(123,97,255,.30), transparent 70%),'
    'radial-gradient(480px 300px at 50% 105%, rgba(255,122,184,.20), transparent 70%)">'
    '<div>' + mark(1, 340) + '</div>'
    '<div class="word" style="font-size:118px"><span class="rom">rom</span>mate</div>'
    '<div class="tag" style="font-size:26px;text-align:center">your RomM library<br>on your handheld</div></body>')

wide = ('<body style="width:920px;height:430px;display:flex;align-items:center;justify-content:center;gap:48px;'
    'background:#16161E;background-image:radial-gradient(380px 300px at 20% 50%, rgba(123,97,255,.28), transparent 70%),'
    'radial-gradient(420px 260px at 90% 110%, rgba(255,122,184,.16), transparent 70%)">'
    '<div>' + mark(2, 230) + '</div>'
    '<div><div class="word" style="font-size:132px"><span class="rom">rom</span>mate</div>'
    '<div class="tag" style="font-size:24px;margin-top:18px">Sync games from RomM to your handheld</div></div></body>')

pads = [(1180, 60, 300, .55), (1540, 250, 200, .35), (1000, 380, 140, .22), (1700, 40, 120, .25),
        (1430, 440, 110, .18),]
hero = ('<body style="width:1920px;height:620px;position:relative;background:#16161E;'
    'background-image:radial-gradient(900px 520px at 78% 30%, rgba(123,97,255,.34), transparent 70%),'
    'radial-gradient(760px 460px at 98% 100%, rgba(255,122,184,.22), transparent 70%),'
    'radial-gradient(700px 400px at 10% 0%, rgba(123,97,255,.10), transparent 70%),linear-gradient(180deg,#191927,#121219)">'
    + ''.join('<div style="position:absolute;left:%dpx;top:%dpx;opacity:%s">%s</div>' % (x, y, o, mark(100 + i, s))
              for i, (x, y, s, o) in enumerate(pads))
    + '</body>')

logo_page = ('<body style="width:1280px;height:400px;display:flex;align-items:center;justify-content:center;gap:52px;background:transparent">'
    '<div style="filter:drop-shadow(0 16px 40px rgba(123,97,255,.45))">' + mark(3, 300) + '</div>'
    '<div class="word" style="font-size:210px;text-shadow:0 8px 40px rgba(0,0,0,.45)"><span class="rom">rom</span>mate</div></body>')

for name, body in {'capsule': capsule, 'wide': wide, 'hero': hero, 'logo': logo_page}.items():
    open(name + '.html', 'w').write(head + body)
