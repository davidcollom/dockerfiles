#!/usr/bin/env perl
$| = 1;

use URI::Escape;
use LWP;
use JSON;
use Cwd;
use Try::Tiny;
use Log::Log4perl qw(:easy);
my $log_level = $ENV{'LOG_LEVEL'};
$log_level ||= 'INFO';
Log::Log4perl->easy_init(Log::Log4perl::Level::to_priority( $log_level ));
my $logger = get_logger();

$SIG{INT} = sub { die "Caught a sigint $!" };
$SIG{TERM} = sub { die "Caught a sigterm $!" };

my $user = $ENV{'USERNAME'}; ;
my $pw = $ENV{'PASSWORD'}; 
my $download_path = $ENV{'DOWNLOAD_PATH'};
$download_path ||= getcwd;
my $extensions = $ENV{'EXTENSIONS'};
$extensions ||= "pdf,epub,mobi";


INFO "Authenticating as $user";
INFO "Downloading to $download_path";
INFO "Download extentions $extensions";

my @exts = split(',', $extensions);

my $ua = new LWP::UserAgent;

my $req = HTTP::Request->new( POST => "https://services.packtpub.com/auth-v1/users/tokens" );
my $login = {
    username => $user,
    password => $pw
};

my $content = encode_json($login);
$req->content($content);
$req->content_type("application/json");

my $res = $ua->request($req);

my $rinfo  = decode_json( $res->content );
my $atoken = $rinfo->{data}->{access};

if (length($atoken) < 4) {
    ERROR("Token: '" . $atoken . "'");
    ERROR("Authentication Failure: " . $res->content);
    die();
}

INFO "Authentication successfull";

my $offset  = 0;
my $max     = 0;
my $page    = 20;
my $baseurl = "https://services.packtpub.com/entitlements-v1/users/me/products?sort=createdAt:DESC&limit=$page";
my @results = ();

while ( !$max || $total < $max ) {
    INFO "Requesting at offset $offset";
    if ($max) {
        INFO "total $max";
    }

    my $tmpurl = $baseurl . "&offset=$offset";
    my $req = HTTP::Request->new( GET => $tmpurl );
    $req->header( "Authorization" => "Bearer $atoken" );

    my $res   = $ua->request($req);
    my $rinfo = decode_json( $res->content );
    $max = $rinfo->{count};

    my @recs = @{ $rinfo->{data} };
    my $rcnt = scalar(@recs);

    INFO " received $rcnt answers.\n";
    $total += $rcnt;
    push( @results, @recs );

    $offset += $rcnt;
}

INFO sprintf("Found total %d", scalar @results );

foreach my $book (@results) {
    my $name = $book->{productName};
    my $pid  = $book->{productId};

    my $fname = $name;
    $fname =~ s|\s*\[ebook\]\s*$||gio;
    $fname =~ s|/|_|gio;
    $fname =~ tr/[a-z0-9\-\.A-Z ]//cd;

    foreach my $ext (@exts) {
        my $path = "$download_path/${fname}/${fname}.${ext}";
        if ( !-e $path ) {
            mkdir "$download_path/${fname}";
            INFO "need to download ${ext} $name"; 

            my $infourl = "https://services.packtpub.com/products-v1/products/${pid}/files/${ext}";
            my $req = HTTP::Request->new( GET => $infourl );
            $req->header( "Authorization" => "Bearer $atoken" );

            my $infores = $ua->request($req);
            my $pdfinfo = decode_json( $infores->content );

            my $pdfurl = $pdfinfo->{data};
            if ( $pdfurl =~ m|^http(s)+://|o ) {
                INFO "Downloading...";
                DEBUG "$pdfurl";
                my $dlreq = HTTP::Request->new( GET => $pdfurl );
                my $dlres = $ua->request($dlreq);
                if ( $dlres->is_success ) {
                    open( my $out, ">${path}" );
                    print $out $dlres->content;
                    close($out);
                }
                else {
                    ERROR "Failed download: " . $res->as_string . "\n";
                    die(1);
                }
            }
        }
        else {
            WARN "already have $path\n";
        }
    }
}
INFO "Downloaded / Confirmed all Books are in Sync"