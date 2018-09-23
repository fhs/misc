#include <u.h>
#include "/usr/local/plan9/src/cmd/devdraw/x11-inc.h"	// XXX
#include <libc.h>
#include <draw.h>
#include <event.h>
#include <regexp.h>

AUTOLIB(X11);

typedef struct Win Win;
struct Win {
	XWindow n;
	int dirty;
	char *label;
	Rectangle r;
};


XDisplay *dpy;
XWindow root;
Atom net_active_window;
Reprog  *exclude  = nil;
Win *win;
int nwin;
int mwin;
int onwin;
int rows, cols;
Font *font;
Image *lightblue;

enum {
	PAD = 3,
	MARGIN = 5
};

void*
erealloc(void *v, ulong n)
{
	v = realloc(v, n);
	if(v == nil)
		sysfatal("out of memory reallocating %lud", n);
	return v;
}

void*
emalloc(ulong n)
{
	void *v;

	v = malloc(n);
	if(v == nil)
		sysfatal("out of memory allocating %lud", n);
	memset(v, 0, n);
	return v;
}

char*
estrdup(char *s)
{
	int l;
	char *t;

	if (s == nil)
		return nil;
	l = strlen(s)+1;
	t = emalloc(l);
	memcpy(t, s, l);

	return t;
}


char*
getproperty(XWindow w, Atom a)
{
	uchar *p;
	int fmt;
	Atom type;
	ulong n, dummy;

	n = 100;
	p = nil;
	XGetWindowProperty(dpy, w, a, 0, 100L, 0, 
		AnyPropertyType, &type, &fmt,
		&n, &dummy, &p);
	if(p == nil || *p == 0)
		return nil;
	return strdup((char*)p);
}

XWindow
findname(XWindow w)
{
	int i;
	uint nxwin;
	XWindow dw1, dw2, *xwin;

	if(getproperty(w, XA_WM_NAME))
		return w;
	if(!XQueryTree(dpy, w, &dw1, &dw2, &xwin, &nxwin))
		return 0;
	for(i=0; i<nxwin; i++)
		if((w = findname(xwin[i])) != 0)
			return w;
	return 0;
}

int
wcmp(const void *w1, const void *w2)
{
	return *(XWindow*)w1 - *(XWindow*)w2;
}

void
refreshwin(void)
{
	XWindow dw1, dw2, *xwin;
	XClassHint class;
	XWindowAttributes attr;
	char *label;
	int i, nw;
	uint nxwin;

	if(!XQueryTree(dpy, root, &dw1, &dw2, &xwin, &nxwin))
		return;
	qsort(xwin, nxwin, sizeof(xwin[0]), wcmp);

	nw = 0;
	for(i=0; i<nxwin; i++){
		memset(&attr, 0, sizeof attr);
		xwin[i] = findname(xwin[i]);
		if(xwin[i] == 0)
			continue;
		XGetWindowAttributes(dpy, xwin[i], &attr);
		if(attr.width <= 0 || attr.override_redirect || attr.map_state != IsViewable)
			continue;
		if(!XGetClassHint(dpy, xwin[i], &class))
			continue;
		
		label = class.res_name;
		if(exclude != nil && regexec(exclude,label,nil,0))
			continue;

		if(nw < nwin && win[nw].n == xwin[i] && strcmp(win[nw].label, label)==0){
			nw++;
			continue;
		}

		if(nw < nwin){
			free(win[nw].label);
			win[nw].label = nil;
		}
		
		if(nw >= mwin){
			mwin += 8;
			win = erealloc(win, mwin*sizeof(win[0]));
		}
		win[nw].n = xwin[i];
		win[nw].label = estrdup(label);
		win[nw].dirty = 1;
		win[nw].r = Rect(0,0,0,0);
		nw++;
	}
	XFree(xwin);
	while(nwin > nw)
		free(win[--nwin].label);
	nwin = nw;
}

void
drawnowin(int i)
{
	Rectangle r;

	r = Rect(0,0,(Dx(screen->r)-2*MARGIN+PAD)/cols-PAD, font->height);
	r = rectaddpt(rectaddpt(r, Pt(MARGIN+(PAD+Dx(r))*(i/rows),
				MARGIN+(PAD+Dy(r))*(i%rows))), screen->r.min);
	draw(screen, insetrect(r, -1), lightblue, nil, ZP);
}

void
drawwin(int i)
{
	draw(screen, win[i].r, lightblue, nil, ZP);
	_string(screen, addpt(win[i].r.min, Pt(2,0)), display->black, ZP,
		font, win[i].label, nil, strlen(win[i].label), 
		win[i].r, nil, ZP, SoverD);
	border(screen, win[i].r, 1, display->black, ZP);	
	win[i].dirty = 0;
}

int
geometry(void)
{
	int i, ncols, z;
	Rectangle r;

	z = 0;
	rows = (Dy(screen->r)-2*MARGIN+PAD)/(font->height+PAD);
	if(rows*cols < nwin || rows*cols >= nwin*2){
		ncols = nwin <= 0 ? 1 : (nwin+rows-1)/rows;
		if(ncols != cols){
			cols = ncols;
			z = 1;
		}
	}

	r = Rect(0,0,(Dx(screen->r)-2*MARGIN+PAD)/cols-PAD, font->height);
	for(i=0; i<nwin; i++)
		win[i].r = rectaddpt(rectaddpt(r, Pt(MARGIN+(PAD+Dx(r))*(i/rows),
					MARGIN+(PAD+Dy(r))*(i%rows))), screen->r.min);

	return z;
}

void
redraw(Image *screen, int all)
{
	int i;

	all |= geometry();
	if(all)
		draw(screen, screen->r, lightblue, nil, ZP);
	for(i=0; i<nwin; i++)
		if(all || win[i].dirty)
			drawwin(i);
	if(!all)
		for(; i<onwin; i++)
			drawnowin(i);

	onwin = nwin;
}

void
eresized(int new)
{
	if(new && getwindow(display, Refmesg) < 0)
		fprint(2,"can't reattach to window");
	geometry();
	redraw(screen, 1);
}

void
selectwin(XWindow win)
{
	XEvent ev;
	long mask;
	
	memset(&ev, 0, sizeof ev);
	ev.xclient.type = ClientMessage;
	ev.xclient.serial = 0;
	ev.xclient.send_event = True;
	ev.xclient.message_type = net_active_window;
	ev.xclient.window = win;
	ev.xclient.format = 32;
	mask = SubstructureRedirectMask | SubstructureNotifyMask;
	
	XSendEvent(dpy, root, False, mask, &ev);
	XMapRaised(dpy, win);
	XSync(dpy, False);
}

void
click(Mouse m)
{
	int i, j;	

	if(m.buttons == 0 || (m.buttons & ~4))
		return;

	for(i=0; i<nwin; i++)
		if(ptinrect(m.xy, win[i].r))
			break;
	if(i == nwin)
		return;

	do
		m = emouse();
	while(m.buttons == 4);

	if(m.buttons != 0){
		do
			m = emouse();
		while(m.buttons);
		return;
	}

	for(j=0; j<nwin; j++)
		if(ptinrect(m.xy, win[j].r))
			break;
	if(j != i)
		return;

	selectwin(win[i].n);
}

void
usage(void)
{
	fprint(2, "usage: winwatch [-e exclude] [-W winsize] [-f font]\n");
	exits("usage");
}

void
main(int argc, char **argv)
{
	char *fontname;
	int Etimer;
	Event e;

	fontname = "/lib/font/bit/lucsans/unicode.8.font";
	ARGBEGIN{
	case 'W':
		winsize = EARGF(usage());
		break;
	case 'f':
		fontname = EARGF(usage());
		break;
	case 'e':
		exclude = regcomp(EARGF(usage()));
		if(exclude == nil)
			sysfatal("Bad regexp");
		break;
	default:
		usage();
	}ARGEND

	if(argc)
		usage();

	dpy = XOpenDisplay("");
	if(dpy == nil)
		sysfatal("open display: %r");
	
	root = DefaultRootWindow(dpy);
	net_active_window = XInternAtom(dpy, "_NET_ACTIVE_WINDOW", False);
	
	initdraw(0, 0, "winwatch");
	lightblue = allocimagemix(display, DPalebluegreen, DWhite);
	if(lightblue == nil)
		sysfatal("allocimagemix: %r");
	if((font = openfont(display, fontname)) == nil)
		sysfatal("font '%s' not found", fontname);

	refreshwin();
	redraw(screen, 1);
	einit(Emouse|Ekeyboard);
	Etimer = etimer(0, 2500);

	for(;;){
		switch(eread(Emouse|Ekeyboard|Etimer, &e)){
		case Ekeyboard:
			if(e.kbdc==0x7F || e.kbdc=='q')
				exits(0);
			break;
		case Emouse:
			if(e.mouse.buttons)
				click(e.mouse);
			/* fall through  */
		default:	/* Etimer */
			refreshwin();
			redraw(screen, 0);
			break;
		}
	}
}
