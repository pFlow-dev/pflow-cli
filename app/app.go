package app

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/pflow-dev/go-metamodel/v2/codec"
	"github.com/pflow-dev/go-metamodel/v2/metamodel"
	"github.com/pflow-dev/go-metamodel/v2/model"
	"github.com/pflow-dev/go-metamodel/v2/server"
	"html/template"
	"log"
	"net/http"
)

const (
	Banner = `
        _____.__                                          
_______/ ____\  |   ______  _  __  ___  ______.__.________
\____ \   __\|  |  /  _ \ \/ \/ /  \  \/  <   |  |\___   /
|  |_> >  |  |  |_(  <_> )     /    >    < \___  | /    / 
|   __/|__|  |____/\____/ \/\_/ /\ /__/\_ \/ ____|/_____ \
|__|                            \/       \/\/           \/
`
)

type Options struct {
	Port            string
	Host            string
	Url             string
	DbPath          string
	NewRelicLicense string
	NewRelicApp     string
	LoadExamples    bool
	UseSandbox      bool
}

type Server struct {
	App         *server.App
	Logger      *log.Logger
	Options     Options
	Router      *mux.Router
	indexPage   *template.Template
	sandboxPage *template.Template
}

func New(store server.Storage, options Options) *Server {
	s := &Server{
		Options: options,
		Router:  mux.NewRouter(),
	}
	s.App = &server.App{
		Service: s,
		Storage: store,
	}
	s.Logger = log.Default()
	if s.Options.UseSandbox {
	  s.Logger.Printf("Sandbox enabled")
		sandboxSource := s.SandboxTemplateSource()
		s.sandboxPage = template.Must(template.New("sandbox.html").Parse(sandboxSource))
	}
	indexSource := s.IndexTemplateSource()
	s.indexPage = template.Must(template.New("index.html").Parse(indexSource))

	s.Logger.Printf("DBPath: %s\n", s.Options.DbPath)
	s.Logger.Printf("Listening on %s:%s\n", s.Options.Host, s.Options.Port)
	return s
}

func (s *Server) IndexPage() *template.Template {
	return s.indexPage
}

func (s *Server) SandboxPage() *template.Template {
	return s.sandboxPage
}

func (s *Server) PrintLinks(m model.Model, url string) {
	s.Logger.Printf("- model[%d] %s\n", m.ID, m.Title)
	s.Logger.Printf("  %s/p/%s/\n", url, m.IpfsCid)
	s.Logger.Printf("  %s/src/%s.json\n", url, m.IpfsCid)
	s.Logger.Printf("  %s/img/%s.svg\n", url, m.IpfsCid)
}

func (s *Server) ServeHTTP(appHandler http.Handler) {
	s.WrapHandler("/p/", s.App.AppPage)
	s.WrapHandler("/p/{pflowCid}/", s.App.AppPage)
	s.WrapHandler("/img/", s.App.SvgHandler)
	s.WrapHandler("/img/{pflowCid}.svg", s.App.SvgHandler)
	s.WrapHandler("/src/", s.App.JsonHandler)
	s.WrapHandler("/src/{pflowCid}.json", s.App.JsonHandler)
	if s.Options.UseSandbox {
		s.WrapHandler("/sandbox/", s.App.SandboxHandler)
		s.WrapHandler("/sandbox/{pflowCid}/", s.App.SandboxHandler)
	}
	s.Router.PathPrefix("/p").Handler(appHandler)
	err := http.ListenAndServe(s.Options.Host+":"+s.Options.Port, s.Router)
	if err != nil {
		panic(err)
	}
}

func (s *Server) WrapHandler(pattern string, handler server.HandlerWithVars) {
	s.Router.HandleFunc(
		pattern,
		func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			handler(vars, w, r)
		})
}
func (s *Server) Event(eventType string, params map[string]interface{}) {
	data, _ := json.Marshal(params)
	s.Logger.Printf("%s => %s\n", eventType, data)
}
func (s *Server) CheckForModel(hostname string, url string, referrer string) (string, bool) {
	defer func() {
		if r := recover(); r != nil {
			s.Logger.Printf("Recovered from panic in CheckForModel: %v", r)
		}
	}()
	mm := metamodel.New()
	_, foundInUrl := mm.UnpackFromUrl(url, "model.json")
	if foundInUrl {
		zippedData, _ := mm.ZipUrl()
		zippedData = zippedData[3:]
		cid := codec.ToOid(codec.Marshal(zippedData)).String()
		id, err := s.App.Model.Create(cid, zippedData, "Untitled", "", "", referrer)
		if err != nil {
			id = s.App.Model.GetByCid(cid).ID
		}
		linkUrl := "https://" + hostname + "/p/" + cid + "/"
		s.Event("modelUnzipped", map[string]interface{}{
			"id":       id,
			"cid":      cid,
			"link":     linkUrl,
			"referrer": referrer,
		})
		return cid, true
	}
	return "", false
}

func (s *Server) CheckForSnippet(hostname string, url string, referrer string) (string, bool) {
	defer func() {
		if r := recover(); r != nil {
			s.Logger.Printf("Recovered from panic in CheckForSnippet: %v", r)
		}
	}()
	srcUrl := "https://" + hostname + url
	sourceCode, foundInUrl := metamodel.UnzipUrl(srcUrl, "declaration.js")
	if !foundInUrl { // try to convert a model to a snippet
		sourceCode, foundInUrl = metamodel.UnzipUrl(srcUrl, "model.json")
		if !foundInUrl {
			return "", false
		} else {
			sourceCode = "const declaration = " + sourceCode
		}
	}
	cid := codec.ToOid(codec.Marshal(sourceCode)).String()
	zippedCode, _ := metamodel.ToEncodedZip([]byte(sourceCode), "declaration.js")
	_, err := s.App.Storage.Snippet.Create(cid, zippedCode, "", "", "", referrer)
	if err != nil {
		http.Error(nil, "Failed to create snippet", http.StatusInternalServerError)
		return "", false
	}
	res := s.App.Storage.Snippet.GetByCid(cid)
	if res.IpfsCid != cid {
		http.Error(nil, "Failed to load snippet by cid", http.StatusInternalServerError)
		return "", false
	}
	linkUrl := "https://" + hostname + "/sandbox/" + cid + "/"
	s.Event("sandboxUnzipped", map[string]interface{}{
		"id":       res.ID,
		"cid":      cid,
		"link":     linkUrl,
		"referrer": referrer,
	})
	return cid, true
}

func (*Server) GetState(r *http.Request) (state metamodel.Vector, ok bool) {
	q := r.URL.Query()
	rawState := q.Get("state")
	if rawState != "" {
		err := codec.Unmarshal([]byte(rawState), &state)
		if err != nil {
			return state, false
		}
		return state, true
	} else {
		return state, false
	}
}

func (s *Server) IndexTemplateSource() string {
	out := `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/>
	<title>pflow | metamodel explorer</title>
	<meta name="viewport" content="width=device-width,initial-scale=1"/>
	<meta name="theme-color" content="#000000"/>
	<meta name="description" content="pflow metamodel editor"/>
	<link rel="icon" href="/p/favicon.ico"/>
	<link rel="apple-touch-icon" href="/p/logo192.png"/>
	<link rel="manifest" href="/p/manifest.json"/>
	<link href="/p/static/css/main.9e6325dd.css" rel="stylesheet">`
	out += SessionDataScript
	out += `<script defer="defer" src=/p/static/js/main.8954f887.js></script>

</head>
<body>
	<noscript>You need to enable JavaScript to run this app.</noscript>
	<div id="root"></div>
</body></html>`
	return out
}

// NOTE: currenlty we load js code from a CDN - this feature disabled by default
func (s *Server) SandboxTemplateSource() string {
	return SandBoxStaticTemplateHead + SandBoxStaticTemplateRest
}

const (
	SessionDataScript = `<script>
	sessionStorage.cid = "{{.IpfsCid}}";
	sessionStorage.data = "{{.Base64Zipped}}";
</script>`

	SandBoxStaticTemplateHead = `<!DOCTYPE html>
<html lang="en">
<head>
    <title>pflow.dev | js sandbox </title>
    <meta charset="utf-8"/>
    <script src="https://cdn.jsdelivr.net/npm/jquery"></script>
    <script src="https://cdn.jsdelivr.net/npm/jquery.terminal/js/jquery.terminal.min.js"></script>
    <link href="https://cdn.jsdelivr.net/npm/jquery.terminal/css/jquery.terminal.min.css" rel="stylesheet"/>
    <script src="https://cdn.jsdelivr.net/npm/ace-builds@1.16.0/src-min-noconflict/ace.min.js "></script>
    <script src="https://cdn.jsdelivr.net/npm/jszip@3.10.1/dist/jszip.min.js"></script>
    <link href="https://cdn.jsdelivr.net/gh/pFlow-dev/pflow-js@v1.0.2/styles/pflow.css" rel="stylesheet"/>`

	SandBoxStaticTemplateRest = `
    <script src="https://cdn.jsdelivr.net/gh/pflow-dev/pflow-js@v1.0.2/src/pflow.js"></script>
    <script>
        defaultPflowSandboxOptions.vim = false;
    </script>
</head>
<body onload=(runPflowSandbox())>
<table id="cdn-stats">
<tr><td>
<a href="https://www.jsdelivr.com/package/gh/pflow-dev/pflow-js" rel="nofollow noopener noreferrer" class="router-ignore"><img alt="" src="https://data.jsdelivr.com/v1/package/gh/pflow-dev/pflow-js/badge" loading="lazy"></a>
</td><td>
&nbsp;&nbsp;&nbsp;
</td><td>
<iframe src="https://ghbtns.com/github-btn.html?user=pFlow-dev&repo=pflow-js&type=star&count=true&size=large" frameborder="0" scrolling="0" width="130" height="40" title="GitHub">
</iframe>
</td></tr>
</table>
<table id="heading">
<tr><td>
<a class="pflow-link" target="_blank" href="./">
    <svg id="logo-header" width="45" height="45" xmlns="http://www.w3.org/2000/svg" xml:space="preserve" style="enable-background:new 0 0 194.9 47.8" viewBox="0 0 47.8 47.8"><path d="M24.57 35.794c.177-.033.433-.032.57.002.135.035-.01.062-.322.06-.313 0-.424-.03-.247-.062zM11.585 28.84c0-1.102.022-1.553.05-1.002.026.551.026 1.454 0 2.005-.028.551-.05.1-.05-1.003zM4.05 23.932c0-1.064.023-1.481.05-.927.027.555.027 1.426 0 1.936-.028.51-.05.056-.05-1.009zm2.965-2.79a1.2 1.2 0 0 1 .497 0c.137.034.025.062-.249.062-.273 0-.385-.028-.248-.063zm7.924-5.479c0-.024.208-.226.462-.45l.462-.405-.418.449c-.389.419-.506.513-.506.406zm-4.406-5.418c.265-.266.514-.484.553-.484.039 0-.146.218-.411.484s-.514.484-.554.484c-.039 0 .146-.218.412-.484z" style="fill:#d0d0d0;fill-opacity:1;stroke-width:.140185" transform="matrix(1.05551 0 0 1.0735 -1.74 -1.355)"/><path d="M11.563 26.421c.002-.304.031-.412.065-.24s.033.42-.003.553c-.035.132-.063-.009-.062-.313zm16.618-2.42c0-.19.032-.267.071-.172a.52.52 0 0 1 0 .345c-.04.095-.071.018-.071-.172zm-12.745-8.018c.142-.152.29-.277.329-.277.039 0-.045.125-.186.277-.142.152-.29.276-.329.276-.039 0 .045-.124.186-.276zm9.135-4.107c.176-.034.432-.032.568.002.136.035-.009.062-.321.06-.313-.001-.424-.03-.247-.062z" style="fill:#b9b9b9;fill-opacity:1;stroke-width:.140185" transform="matrix(1.05551 0 0 1.0735 -1.74 -1.355)"/><path d="M11.55 25.66c0-.19.032-.267.072-.172a.52.52 0 0 1 0 .345c-.04.095-.072.018-.072-.172zm25.74-1.728c.003-.304.032-.412.066-.24s.032.42-.003.553c-.036.132-.064-.008-.062-.313zm-29.2-2.79c.102-.04.225-.035.272.01.047.047-.037.08-.187.074-.165-.007-.199-.04-.085-.084z" style="fill:#a3a3a3;fill-opacity:1;stroke-width:.140185" transform="matrix(1.05551 0 0 1.0735 -1.74 -1.355)"/><path d="m9.804 37.309-.195-.242.248.19c.137.104.25.213.25.241 0 .114-.118.04-.303-.19zm1.746-12.201c0-.19.032-.268.072-.173a.52.52 0 0 1 0 .345c-.04.095-.072.018-.072-.172zm-3.034-3.966c.103-.04.225-.035.272.01.048.047-.036.08-.186.074-.166-.007-.2-.04-.086-.084z" style="fill:#8c8c8c;fill-opacity:1;stroke-width:.140185" transform="matrix(1.05551 0 0 1.0735 -1.74 -1.355)"/><path d="M19.226 40.592c0-.19.032-.268.071-.173a.52.52 0 0 1 0 .346c-.04.095-.071.017-.071-.173z" style="fill:#767676;fill-opacity:1;stroke-width:.140185" transform="matrix(1.05551 0 0 1.0735 -1.74 -1.355)"/><path d="M19.214 41.065c.007-.162.04-.194.086-.084.041.1.036.219-.01.265-.048.046-.082-.036-.076-.181zm.028-1.302c0-.343.028-.483.062-.311.033.17.033.45 0 .622-.034.17-.062.03-.062-.311zM44.089 24.75c.007-.16.04-.193.086-.083.041.1.036.219-.011.265-.047.046-.081-.036-.075-.182zm-21.332-2.874c0-.03.112-.138.248-.242l.25-.19-.196.242c-.186.23-.302.303-.302.19zm-13.53-.734c.102-.04.225-.035.272.01.047.047-.037.08-.186.074-.166-.007-.2-.04-.086-.084z" style="fill:#606060;fill-opacity:1;stroke-width:.140185" transform="matrix(1.05551 0 0 1.0735 -1.74 -1.355)"/><path d="M19.244 38.587c.001-.38.029-.519.061-.309.033.21.032.521-.002.692-.033.17-.06-.002-.06-.383zm14.173-5.928c0-.028.112-.137.249-.242l.249-.19-.195.243c-.186.23-.303.303-.303.19zm10.694-8.52c0-.265.03-.374.065-.241.036.133.036.35 0 .484-.036.133-.065.024-.065-.242zm-.022-.771c.007-.161.04-.194.086-.084.041.1.036.22-.011.265-.047.046-.081-.035-.075-.181zM9.653 21.142c.103-.04.225-.035.273.01.047.047-.037.08-.187.074-.166-.007-.2-.04-.086-.084zM37.521 9.519l-.195-.242.249.19c.237.18.312.294.195.294-.03 0-.141-.109-.249-.242zM23.156 4.551c.103-.04.226-.035.273.011.047.046-.037.079-.187.073-.165-.007-.199-.04-.086-.084z" style="fill:#494949;fill-opacity:1;stroke-width:.140185" transform="matrix(1.05551 0 0 1.0735 -1.74 -1.355)"/><path d="M22.293 43.077c-1.266-.14-3.605-.666-4.945-1.113-5.51-1.835-10.571-6.878-12.408-12.363-.716-2.138-.8-2.733-.802-5.709-.002-3.09.133-4.032.871-6.089 1.558-4.342 4.184-7.533 8.422-10.233 2.703-1.723 5.62-2.658 9.153-2.933 3.997-.31 7.31.333 10.887 2.117 3.071 1.532 6.032 4.227 7.814 7.111 1.97 3.19 2.807 6.176 2.802 9.985-.004 2.959-.353 4.778-1.378 7.186-2.039 4.79-6.483 8.972-11.495 10.815-2.816 1.035-6.31 1.516-8.921 1.226zm-3.057-1.448c.067-.228.08-1.892.028-3.698-.052-1.806-.063-3.284-.024-3.284.038 0 .35.12.691.265.343.145 1.39.432 2.328.637l1.706.373 1.635-.079c2.01-.097 2.93-.327 4.78-1.199 2.24-1.054 3.708-2.327 5.122-4.445 2.557-3.83 2.485-8.985-.182-12.921-2.456-3.625-6.56-5.614-11.142-5.398-3.473.163-5.736 1.2-8.464 3.877-1.57 1.541-2.5 2.87-3.187 4.565-.917 2.257-.9 2.07-.954 10.316l-.05 7.535 1.177.847c1.73 1.245 3.291 2.13 4.655 2.639 1.437.535 1.715.53 1.88-.03zm4.825-14.735c-.735-.219-1.674-1.223-1.932-2.066-.592-1.94.91-3.874 3.004-3.865 1.718.008 3.025 1.362 3.025 3.133 0 .862-.655 1.972-1.437 2.436-.736.437-1.883.593-2.66.362zm-13.69-5.748 1.305-.09.55-1.141c1.527-3.17 3.556-5.351 6.497-6.987.694-.386 1.338-.774 1.431-.863.131-.124.124-.772-.03-2.857-.11-1.483-.202-2.824-.205-2.98a1.044 1.044 0 0 0-.183-.52l-.177-.235-1.06.286c-1.447.39-4.04 1.612-5.69 2.68-1.448.938-3.48 2.868-4.745 4.505-1.539 1.993-3.742 7.44-3.294 8.144.106.168 3.503.203 5.602.058z" style="fill:#333;fill-opacity:1;stroke-width:.140185" transform="matrix(1.05551 0 0 1.0735 -1.74 -1.355)"/></svg>
</a>
</td><td>
   <div class="tooltip">
       <button id="simulate" class="btn">
           <svg width="12" height="14">
           <g transform="translate(-2,0) scale(.7,.7)">
           <path d="M8 5v14l11-7z"></path>
           </g>
           </svg>
       Simulate</button>
       <span class="tooltiptext">{Ctl+Enter} to run model</span>
   </div>
  <div class="tooltip">
   <button id="download" class="btn">
       <svg width="12" height="14">
       <g transform="translate(-2,0) scale(.66,.66)">
       <path d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z"></path>
       </g>
       </svg> Download</button>
     <span class="tooltiptext">download.zip</span>
   </div>
  <div class="tooltip">
  <a id="share" target=_blank >
  <button id="permalink" class="btn">
     <svg width="18" height="14">
     <g transform="scale(.8,.8)">
     <path d="M3.9 12c0-1.71 1.39-3.1 3.1-3.1h4V7H7c-2.76 0-5 2.24-5 5s2.24 5 5 5h4v-1.9H7c-1.71 0-3.1-1.39-3.1-3.1zM8 13h8v-2H8v2zm9-6h-4v1.9h4c1.71 0 3.1 1.39 3.1 3.1s-1.39 3.1-3.1 3.1h-4V17h4c2.76 0 5-2.24 5-5s-2.24-5-5-5z"></path>
     </g>
     </svg> Link
     </button>
     </a>
     <span class="tooltiptext">copy link to clipboard</span>
  </div>
  <div class="tooltip">
  <button id="embed" class="btn">
     <svg width="18" height="14">
     <g transform="translate(2,0) scale(.6,.6)">
     <path d="M18 16.08c-.76 0-1.44.3-1.96.77L8.91 12.7c.05-.23.09-.46.09-.7s-.04-.47-.09-.7l7.05-4.11c.54.5 1.25.81 2.04.81 1.66 0 3-1.34 3-3s-1.34-3-3-3-3 1.34-3 3c0 .24.04.47.09.7L8.04 9.81C7.5 9.31 6.79 9 6 9c-1.66 0-3 1.34-3 3s1.34 3 3 3c.79 0 1.5-.31 2.04-.81l7.12 4.16c-.05.21-.08.43-.08.65 0 1.61 1.31 2.92 2.92 2.92 1.61 0 2.92-1.31 2.92-2.92s-1.31-2.92-2.92-2.92z"></path>
     </g>
     </svg> Embed</button>
     <span class="tooltiptext">copy iframe widget source</span>
  </div>
  <a href="https://pflow.dev/help" target="_blank">
  <button id="help" class="btn">
     <svg width="18" height="14">
     <g transform="translate(0,1) scale(.6,.6)">
      <path d="M11 18h2v-2h-2v2zm1-16C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm0-14c-2.21 0-4 1.79-4 4h2c0-1.1.9-2 2-2s2 .9 2 2c0 2-3 1.75-3 5h2c0-2.25 3-2.5 3-5 0-2.21-1.79-4-4-4z"></path>
     </g>
     </svg>Help</button>
</a>
</td><td>
    <a href="?z=" id="editLink" target="_blank">edit</a>
    <input type="checkbox" id="viewCode" class="feature-flag" checked>Code</input>
    <input type="checkbox" id="viewTerminal" class="feature-flag" checked>Terminal</input>
</td></tr>
</table>
<canvas id="pflow-canvas" height="600px" width="1116px"></canvas>
<pre id="editor">{{.SourceCode}}</pre>
<pre id="term"><a class="pflow-link" target="_blank" href="https://pflow.dev/about">pflow.dev petri-net editor</a></pre>
</body>
</html>`
)
