package fr.ialocale.mobile;
import android.app.Activity;
import android.os.Bundle;
import android.content.Intent;
import android.content.Context;
import android.graphics.Color;
import android.content.res.ColorStateList;
import android.graphics.drawable.Drawable;
import android.graphics.drawable.GradientDrawable;
import android.graphics.drawable.RippleDrawable;
import android.graphics.drawable.StateListDrawable;
import android.net.*;
import android.view.View;
import android.widget.*;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;
public final class MainActivity extends Activity {
    private static final int PICK=40;
    private final ExecutorService worker=Executors.newSingleThreadExecutor();
    private final AtomicBoolean cancelled=new AtomicBoolean();
    private boolean nativeReady=false,busy=false, code=false;
    private TextView state, modelLabel, output, connection;
    private EditText prompt;
    private Button run, stop, pick, download, textTab, codeTab;
    private File model;
    private Conversations conversations;
    private String thread;
    private LinearLayout messages;
    private TextView brand;
    private android.app.Dialog drawer;
    private String workshop="Texte";
    private final java.util.Map<String,Button> tabs=new java.util.LinkedHashMap<>();
    private File currentMedia,exporting;
    private ImageView imagePreview;
    private VideoView videoPreview;
    private Button saveMedia,playMedia;
    private android.media.MediaPlayer player;
    private String category(){return workshop;}
    private boolean media(){return !("Texte".equals(workshop)||"Code".equals(workshop));}
    private File creation(String id){return new File(new File(getFilesDir(),"creations"),id);}
    private void preview(File file){
        currentMedia=file;if(player!=null){player.release();player=null;}videoPreview.stopPlayback();
        videoPreview.setVisibility(View.GONE);imagePreview.setImageDrawable(null);imagePreview.setVisibility(View.GONE);
        playMedia.setVisibility(View.GONE);saveMedia.setVisibility(file==null?View.GONE:View.VISIBLE);
        if(file==null)return;
        if(file.getName().endsWith(".png")){
            imagePreview.setImageBitmap(android.graphics.BitmapFactory.decodeFile(file.getPath()));imagePreview.setVisibility(View.VISIBLE);
        }else{
            playMedia.setVisibility(View.VISIBLE);
            if(file.getName().endsWith(".mp4")){videoPreview.setVisibility(View.VISIBLE);videoPreview.setVideoPath(file.getPath());videoPreview.seekTo(1);}
        }
    }
    private void play(){
        if(currentMedia==null)return;
        if(currentMedia.getName().endsWith(".mp4")){videoPreview.seekTo(0);videoPreview.start();return;}
        try{if(player!=null)player.release();player=new android.media.MediaPlayer();player.setDataSource(currentMedia.getPath());player.prepare();player.start();}
        catch(Exception e){state.setText("Lecture impossible · "+e.getMessage());}
    }
    private void export(){
        if(currentMedia==null||busy)return;
        exporting=currentMedia;Intent i=new Intent(Intent.ACTION_CREATE_DOCUMENT);i.addCategory(Intent.CATEGORY_OPENABLE);
        i.setType(workshop.equals("Image")?"image/png":workshop.equals("Vidéo")?"video/mp4":"audio/wav");
        i.putExtra(Intent.EXTRA_TITLE,workshop+"-"+thread+"."+MediaWorkshop.extension(workshop));startActivityForResult(i,43);
    }
    private String display(File f){return getPreferences(0).getString("name-"+f.getName(),f.getName());}
    private void models(){
        if(busy)return;
        File[] files=new File(getFilesDir(),"models").listFiles((d,n)->n.endsWith(".gguf"));
        if(files==null||files.length==0){state.setText("Aucun modèle installé.");return;}
        String[] names=new String[files.length];
        for(int i=0;i<files.length;i++)names[i]=display(files[i])+" · "+files[i].length()/1048576+" Mio";
        new android.app.AlertDialog.Builder(this).setTitle("Modèles · "+getFilesDir().getUsableSpace()/1048576+" Mio libres")
            .setItems(names,(dialog,index)->{
                File f=files[index];
                new android.app.AlertDialog.Builder(this).setTitle(names[index])
                .setPositiveButton("Utiliser",(a,b)->{model=f;getPreferences(0).edit().putString("model",f.getName()).apply();refresh();})
                .setNegativeButton("Supprimer définitivement",(a,b)->{
                    if(!f.delete()){state.setText("Suppression impossible.");return;}
                    if(f.equals(model)){model=null;getPreferences(0).edit().remove("model").apply();}
                    getPreferences(0).edit().remove("name-"+f.getName()).apply();refresh();
                }).setNeutralButton("Annuler",null).show();
            }).show();
    }
    private void newConversation() {
        thread = null;
        preview(null);
        output.setText("");
        messages.removeAllViews();
        prompt.setText("");
        state.setText("");
        brand.setText(workshop.equals("Texte") ? getString(R.string.app_name) : workshop);
    }

    private void renderMessages() {
        messages.removeAllViews();
        for (String[] turn : conversations.turns(thread)) {
            TextView question = label(turn[0], 16);
            question.setTextIsSelectable(true);
            question.setPadding(dp(16), dp(10), dp(16), dp(10));
            question.setBackground(glassGradient(18, 0xff49405d, 0xff403650));
            LinearLayout.LayoutParams bubble = new LinearLayout.LayoutParams(-2, -2);
            bubble.gravity = android.view.Gravity.END;
            bubble.setMargins(dp(34), dp(12), 0, dp(6));
            messages.addView(question, bubble);
            TextView answer = label(turn[1], 17);
            answer.setTextIsSelectable(true);
            answer.setLineSpacing(dp(3), 1);
            messages.addView(answer);
            if (messages.getChildCount() == 2) brand.setText(turn[0]);
        }
    }

    private void history() {
        if (busy) return;
        drawer = new android.app.Dialog(this);
        LinearLayout panel = column();
        panel.setBackgroundColor(0xff211e29);
        panel.setPadding(dp(14), dp(16), dp(14), dp(12));
        panel.setFocusableInTouchMode(true);
        panel.addView(label("Conversations", 22));
        panel.addView(createWorkshopBar());
        select(workshop);
        EditText search = new EditText(this);
        search.setSingleLine();
        search.setHint("Rechercher une discussion");
        panel.addView(search);
        panel.addView(button("Nouvelle discussion", v -> { newConversation(); drawer.dismiss(); }));
        ScrollView list = new ScrollView(this);
        LinearLayout rows = column();
        list.addView(rows);
        panel.addView(list, new LinearLayout.LayoutParams(-1, 0, 1));
        java.util.ArrayList<String[]> entries = conversations.list(category());
        Runnable filter = () -> {
            rows.removeAllViews();
            String term = search.getText().toString().toLowerCase(java.util.Locale.ROOT);
            for (String[] row : entries) {
                if (!row[1].toLowerCase(java.util.Locale.ROOT).contains(term)) continue;
                Button item = button(row[1], v -> {
                    newConversation();
                    thread = row[0];
                    renderMessages();
                    if (media()) preview(new File(creation(thread), "creation." + MediaWorkshop.extension(workshop)));
                    drawer.dismiss();
                });
                item.setSingleLine();
                item.setEllipsize(android.text.TextUtils.TruncateAt.END);
                item.setGravity(android.view.Gravity.START | android.view.Gravity.CENTER_VERTICAL);
                item.setOnLongClickListener(v -> {
                    new android.app.AlertDialog.Builder(this).setMessage("Supprimer cette discussion ?")
                        .setNegativeButton("Annuler", null).setPositiveButton("Supprimer", (d, i) -> {
                            if (media() && !Attachments.erase(creation(row[0]))) {
                                drawer.dismiss(); state.setText("Suppression du fichier impossible."); return;
                            }
                            conversations.remove(row[0]);
                            if (row[0].equals(thread)) newConversation();
                            drawer.dismiss(); history();
                        }).show();
                    return true;
                });
                rows.addView(item);
            }
            if (rows.getChildCount() == 0) rows.addView(label(term.isEmpty() ? "Aucune discussion dans cet atelier" : "Aucun résultat", 14));
        };
        search.addTextChangedListener(new android.text.TextWatcher() {
            public void beforeTextChanged(CharSequence s, int start, int count, int after) {}
            public void onTextChanged(CharSequence s, int start, int before, int count) { filter.run(); }
            public void afterTextChanged(android.text.Editable e) {}
        });
        filter.run();
        drawer.setContentView(panel);
        android.view.Window window = drawer.getWindow();
        window.setBackgroundDrawableResource(android.R.color.transparent);
        window.setGravity(android.view.Gravity.START);
        window.setLayout(Math.min(dp(380), getResources().getDisplayMetrics().widthPixels - dp(32)), -1);
        window.setSoftInputMode(android.view.WindowManager.LayoutParams.SOFT_INPUT_STATE_ALWAYS_HIDDEN
                | android.view.WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
        drawer.show();
    }
    private static native byte[] generateNative(String path, byte[] input, int maxTokens, int threads, String grammar);
    private static native void resetStop();
    private static native void stopNative();
    private int dp(int x){return (int)(getResources().getDisplayMetrics().density*x+.5f);}
    private TextView label(String s,int size){TextView t=new TextView(this);t.setText(s);t.setTextSize(size);t.setTextColor(Color.WHITE);t.setPadding(0,dp(8),0,dp(8));return t;}
    private Button button(String s,View.OnClickListener listener){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setPadding(dp(8),0,dp(8),0);b.setMinWidth(0);b.setMinimumWidth(0);b.setBackground(new RippleDrawable(ColorStateList.valueOf(0x30ffffff),glassGradient(22,0xff373240,0xff302c38),null));b.setOnClickListener(listener);return b;}
    @Override public void onCreate(Bundle savedState){super.onCreate(savedState);
        try{System.loadLibrary("ialocale");nativeReady=true;}catch(LinkageError e){nativeReady=false;}
        conversations=new Conversations(this);String previous=getPreferences(0).getString("model","");if(!previous.isEmpty()){File f=new File(getFilesDir(),"models/"+new File(previous).getName());if(f.isFile()) model=f;}
        LinearLayout screen = column();
        getWindow().setStatusBarColor(0xff11121a);
        getWindow().setNavigationBarColor(0xff11121a);
        screen.setBackgroundColor(0xff11121a);
        screen.setPadding(dp(12), dp(6), dp(12), dp(8));
        screen.setFitsSystemWindows(true);
        getWindow().setSoftInputMode(android.view.WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
        LinearLayout header = new LinearLayout(this);
        header.setGravity(android.view.Gravity.CENTER_VERTICAL);
        Button historyButton = button("☰", v -> history());
        historyButton.setContentDescription("Historique");
        header.addView(historyButton, new LinearLayout.LayoutParams(dp(48), dp(48)));
        brand = label(getString(R.string.app_name), 19);
        Drawable emblem = getDrawable(R.drawable.studio_icon);
        emblem.setBounds(0, 0, dp(28), dp(28));
        brand.setCompoundDrawablesRelative(emblem, null, null, null);
        brand.setSingleLine();
        brand.setEllipsize(android.text.TextUtils.TruncateAt.END);
        brand.setCompoundDrawablePadding(dp(4));
        brand.setGravity(android.view.Gravity.CENTER);
        header.addView(brand, new LinearLayout.LayoutParams(0, -2, 1));
        Button fresh = button("＋", v -> {
            if (busy) return;
            newConversation();
        });
        fresh.setContentDescription("Nouvelle discussion");
        header.addView(fresh, new LinearLayout.LayoutParams(dp(48), dp(48)));
        screen.addView(header);
        header.setBackground(glassGradient(24, 0xff302b3b, 0xff24212d));
        modelLabel = label("", 14);
        modelLabel.setGravity(android.view.Gravity.CENTER);
        modelLabel.setMinHeight(dp(48));
        modelLabel.setOnClickListener(v -> showModelMenu());
        modelLabel.setFocusable(true);
        modelLabel.setSingleLine();
        modelLabel.setEllipsize(android.text.TextUtils.TruncateAt.END);
        ScrollView scroll = new ScrollView(this);
        scroll.setFillViewport(true);
        LinearLayout root = column();
        root.setPadding(dp(12), dp(4), dp(12), dp(8));
        scroll.addView(root);
        screen.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));
        connection = label("", 12);
        root.addView(connection);
        state = label("", 14);
        root.addView(state);
        output = label("", 17);
        output.setLineSpacing(dp(3), 1);
        output.setTextIsSelectable(true);
        messages = column();
        root.addView(messages);
        root.addView(output);
        pick=button("Importer un modèle GGUF",v->{if(busy)return;Intent i=new Intent(Intent.ACTION_OPEN_DOCUMENT);i.setType("*/*");i.addCategory(Intent.CATEGORY_OPENABLE);startActivityForResult(i,PICK);});
        download=button("Télécharger un modèle sur Hugging Face",v->{refreshNetwork();if(!online()){state.setText("Vous êtes hors ligne. Connectez-vous pour télécharger un modèle. Les modèles installés restent utilisables.");return;}try{startActivity(new Intent(Intent.ACTION_VIEW,Uri.parse("https://huggingface.co/models?library=gguf")));}catch(Exception e){state.setText("Aucun navigateur disponible.");}});
        Button attachment = button("＋", v -> {
            if (busy) return;
            Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
            intent.setType("*/*");
            intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[]{"text/*","application/json","image/*","audio/*","video/*"});
            startActivityForResult(intent, 41);
        });
        attachment.setContentDescription("Joindre un document, une image ou un audio");
        speechAction = button("Modèle de transcription Vosk", v->{
            if(busy)return;
            File vocal=Attachments.speechModel(this);
            if(vocal.isDirectory()){
                new android.app.AlertDialog.Builder(this).setMessage("Supprimer le modèle vocal installé ?")
                .setPositiveButton("Supprimer",(a,b)->state.setText(Attachments.erase(vocal)?"Modèle vocal supprimé.":"Suppression incomplète.")).setNegativeButton("Annuler",null).show();
            }else{Intent i=new Intent(Intent.ACTION_OPEN_DOCUMENT);i.setType("*/*");startActivityForResult(i,42);}
        });
        LinearLayout composer = column();
        composer.setPadding(dp(10), dp(5), dp(10), dp(6));
        composer.setBackground(glassGradient(22, 0xff302b42, 0xff201f2d));
        prompt = new EditText(this);
        prompt.setBackgroundColor(Color.TRANSPARENT);
        prompt.setTextColor(Color.WHITE);
        prompt.setHintTextColor(0xffc5bdce);
        prompt.setMinLines(1);
        prompt.setMaxLines(3);
        prompt.setInputType(android.text.InputType.TYPE_CLASS_TEXT | android.text.InputType.TYPE_TEXT_FLAG_MULTI_LINE);
        composer.addView(prompt);
        LinearLayout actions = new LinearLayout(this);
        actions.addView(attachment, new LinearLayout.LayoutParams(dp(48), dp(48)));
        actions.addView(modelLabel, new LinearLayout.LayoutParams(0, dp(48), 1));
        run = button("↑", v -> start(false));
        run.setContentDescription("Générer localement");
        actions.addView(run, new LinearLayout.LayoutParams(dp(48), dp(48)));
        stop = button("Arrêter", v -> {cancelled.set(true);if(nativeReady)stopNative();state.setText("Arrêt demandé…");});
        actions.addView(stop);
        composer.addView(actions);
        screen.addView(composer);
        screen.setFocusableInTouchMode(true);
        screen.requestFocus();
        imagePreview=new ImageView(this);imagePreview.setAdjustViewBounds(true);root.addView(imagePreview);
        videoPreview=new VideoView(this);root.addView(videoPreview,new LinearLayout.LayoutParams(-1,dp(220)));
        playMedia=button("Lire la création",v->play());root.addView(playMedia);
        saveMedia=button("Exporter la création",v->export());root.addView(saveMedia);
        setContentView(screen);preview(null);select("Texte");refresh();refreshNetwork();
    }
    private Button speechAction;

    private LinearLayout column() {
        LinearLayout layout = new LinearLayout(this);
        layout.setOrientation(LinearLayout.VERTICAL);
        return layout;
    }

    private void showModelMenu() {
        if (busy) return;
        String[] items = {"Importer un modèle GGUF", "Gérer les modèles", "Télécharger un modèle sur Hugging Face",
                          "Modèle de transcription Vosk", "Tester hors ligne"};
        new android.app.AlertDialog.Builder(this).setTitle("Modèles locaux").setItems(items, (dialog, index) -> {
            switch (index) {
                case 0: pick.performClick(); break;
                case 1: models(); break;
                case 2: download.performClick(); break;
                case 3: speechAction.performClick(); break;
                case 4:
                    if (model == null) state.setText("Importez un modèle GGUF pour le test.");
                    else start(true);
                    break;
            }
        }).setNegativeButton("Fermer", null).show();
    }

    private LinearLayout createWorkshopBar() {
        LinearLayout bar = new LinearLayout(this);
        bar.setOrientation(LinearLayout.HORIZONTAL);
        bar.setPadding(dp(6), dp(6), dp(6), dp(6));
        bar.setBackground(glassGradient(24, 0xff4b435f, 0xff251f35, 0xff302940));
        bar.setElevation(dp(5));
        HorizontalScrollView strip = new HorizontalScrollView(this);
        strip.setHorizontalScrollBarEnabled(false);
        strip.setFillViewport(true);
        strip.addView(bar);
        LinearLayout container = column();
        container.addView(strip);
        for(String title:new String[]{"Texte","Code","Image","Vidéo","Musique"}){
            Button tab=workshopButton(title,0xff544c78,0xff302940);
            tab.setTextSize(14);tab.setMinWidth(dp(58));tab.setMinimumWidth(dp(58));bar.addView(tab,new LinearLayout.LayoutParams(-2,-2,1));tabs.put(title,tab);
        }
        return container;
    }

    private Button workshopButton(String title, int highlight, int base) {
        Button tab = button(title, view -> { select(title); drawer.dismiss(); });
        tab.setMinHeight(dp(48));
        tab.setMinimumHeight(dp(48));
        tab.setPadding(dp(8), dp(7), dp(8), dp(7));
        tab.setTextColor(new ColorStateList(
                new int[][]{{-android.R.attr.state_enabled}, {android.R.attr.state_selected}, {}},
                new int[]{0xffa49bb4, Color.WHITE, 0xffe4ddef}));
        tab.setBackgroundTintList(null);
        tab.setStateListAnimator(null);
        StateListDrawable surface = new StateListDrawable();
        surface.addState(new int[]{android.R.attr.state_selected}, glassGradient(18, highlight, base));
        surface.addState(new int[]{}, glassGradient(18, Color.TRANSPARENT, Color.TRANSPARENT));
        Drawable mask = glassGradient(18, Color.WHITE, Color.WHITE);
        tab.setBackground(new RippleDrawable(ColorStateList.valueOf(0x36ffffff), surface, mask));
        return tab;
    }

    private GradientDrawable glassGradient(int radius, int... colors) {
        GradientDrawable surface = new GradientDrawable(GradientDrawable.Orientation.TL_BR, colors);
        surface.setCornerRadius(dp(radius));
        return surface;
    }

    private void select(String selected) {
        if(!workshop.equals(selected)){newConversation();}
        workshop=selected;code=selected.equals("Code");
        for(java.util.Map.Entry<String,Button> tab:tabs.entrySet()){
            tab.getValue().setSelected(selected.equals(tab.getKey()));tab.getValue().setEnabled(!busy);
        }
        String hint=selected.equals("Image")?"Décrivez une illustration vectorielle…":selected.equals("Vidéo")?"Décrivez une scène animée de 3 secondes…":selected.equals("Musique")?"Décrivez une mélodie instrumentale…":"Écrire une demande…";
        prompt.setHint(hint);
        if (thread == null) brand.setText(workshop.equals("Texte") ? getString(R.string.app_name) : workshop);
    }
    private boolean online(){ConnectivityManager cm=(ConnectivityManager)getSystemService(Context.CONNECTIVITY_SERVICE);NetworkCapabilities caps=cm.getNetworkCapabilities(cm.getActiveNetwork());return caps!=null&&caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED);}
    private void refreshNetwork(){connection.setText(online()?"En ligne · calcul toujours local":"Hors ligne · modèles installés utilisables");}
    @Override protected void onResume(){super.onResume();if(connection!=null)refreshNetwork();}
    private void refresh(){modelLabel.setText(model==null?"Choisir un modèle ▾":display(model)+" ▾");modelLabel.setContentDescription("Modèles locaux");modelLabel.setEnabled(!busy);run.setVisibility(busy?View.GONE:View.VISIBLE);stop.setVisibility(busy?View.VISIBLE:View.GONE);run.setEnabled(nativeReady&&!busy&&model!=null);pick.setEnabled(!busy);download.setEnabled(!busy);stop.setEnabled(busy);select(workshop);if(!nativeReady)state.setText("Moteur natif non chargé. La génération est indisponible.");}
    @Override protected void onActivityResult(int req,int result,Intent data){super.onActivityResult(req,result,data);
        if(req==43){
            File source=exporting;exporting=null;
            if(result!=RESULT_OK||source==null||data==null||data.getData()==null)return;
            try(OutputStream out=getContentResolver().openOutputStream(data.getData(),"wt")){
                if(out==null)throw new IOException("Destination inaccessible");java.nio.file.Files.copy(source.toPath(),out);state.setText("Création exportée.");
            }catch(Exception e){state.setText("Export impossible · "+e.getMessage());}return;
        }
        if((req==41||req==42)&&result==RESULT_OK&&data!=null&&data.getData()!=null&&!busy){
        Uri uri=data.getData();busy=true;cancelled.set(false);refresh();state.setText("Lecture locale en cours…");
        worker.submit(()->{try{
            if(req==42){Attachments.installSpeech(this,uri,cancelled::get);runOnUiThread(()->finishJob("Modèle vocal installé."));return;}
            String mime=getContentResolver().getType(uri);String content;
            if(mime!=null&&mime.startsWith("image/"))content=Attachments.image(this,uri);
            else if(mime!=null&&(mime.startsWith("audio/")||mime.startsWith("video/")))content=Attachments.audio(this,uri,cancelled::get);
            else try(InputStream in=getContentResolver().openInputStream(uri)){
                if(in==null)throw new IOException("Document inaccessible");
                ByteArrayOutputStream bytes=new ByteArrayOutputStream();byte[] buf=new byte[2048];int n;
                while((n=in.read(buf))!=-1){if(bytes.size()+n>24000)throw new IOException("Document trop long");bytes.write(buf,0,n);}
                content=StandardCharsets.UTF_8.newDecoder().decode(java.nio.ByteBuffer.wrap(bytes.toByteArray())).toString();
            }
            String added=content.trim();
            if(added.isEmpty())throw new IOException("Aucun texte extrait.");
            runOnUiThread(()->{if(isDestroyed())return;if(!cancelled.get()&&added.length()+prompt.length()+1<=6000){prompt.append("\n"+added);finishJob("Pièce jointe intégrée.");}else finishJob("Import annulé ou contexte mobile dépassé (6 000 caractères).");});
        }catch(Exception e){runOnUiThread(()->finishJob("Pièce jointe · "+e.getMessage()));}});return;
    }if(req!=PICK||result!=RESULT_OK||data==null||data.getData()==null||busy)return;Uri uri=data.getData();busy=true;cancelled.set(false);refresh();state.setText("Import local en cours…");worker.submit(()->{try{InputStream in=getContentResolver().openInputStream(uri);if(in==null)throw new IOException("Fichier inaccessible");String displayName=uri.getLastPathSegment();try(android.database.Cursor cursor=getContentResolver().query(uri,new String[]{android.provider.OpenableColumns.DISPLAY_NAME},null,null,null)){if(cursor!=null&&cursor.moveToFirst())displayName=cursor.getString(0);}final String filename=displayName;File dest=ModelFiles.copy(in,new File(getFilesDir(),"models"),cancelled::get);runOnUiThread(()->{if(isDestroyed())return;model=dest;getPreferences(0).edit().putString("model",dest.getName()).putString("name-"+dest.getName(),filename).apply();finishJob("Modèle importé.");});}catch(Exception e){runOnUiThread(()->finishJob("Import : "+e.getMessage()));}});}
    private void start(boolean test){if(busy||!nativeReady||model==null)return;String q=test?(media()?(workshop.equals("Musique")?"Une mélodie au piano":"Un soleil sur la mer"):"Réponds OK."):prompt.getText().toString().trim();if(q.isEmpty()){state.setText("Écris une demande.");return;}if(q.length()>6000){state.setText("Demande trop longue pour ce profil.");return;}File selected=model;String past=thread==null||test?"":conversations.read(thread);
    String recent=past.substring(Math.max(0,past.length()-2200));
    boolean creating=media();String kind=workshop;String request=creating?MediaWorkshop.instructions(kind)+q:recent+(code?"Réponds avec du code et des explications utiles :\n"+q:q);busy=true;cancelled.set(false);resetStop();refresh();output.setText("");state.setText("Calcul local… (aucune requête Internet)");worker.submit(()->{try{byte[] result=generateNative(selected.getAbsolutePath(),request.getBytes(StandardCharsets.UTF_8),creating?1024:384,Math.min(4,Runtime.getRuntime().availableProcessors()),creating?MediaWorkshop.grammar(kind):null);String answer=new String(result,StandardCharsets.UTF_8);if(creating){
                    String id=java.util.UUID.randomUUID().toString();File dir=creation(id);
                    File file=MediaWorkshop.render(dir,kind,answer,cancelled::get);
                    try{conversations.add(kind,id,q,file.getName());}catch(Exception e){Attachments.erase(dir);throw e;}
                    runOnUiThread(()->{if(isDestroyed())return;thread=id;output.setText(q);preview(file);finishJob("Création enregistrée · "+file.getName());});
                    return;
                }
                runOnUiThread(()->{if(isDestroyed())return;if(!test){
                    if(thread==null)thread=java.util.UUID.randomUUID().toString();
                    conversations.add(category(),thread,q,answer);renderMessages();if (prompt.getText().toString().trim().equals(q)) prompt.setText("");
                }else output.setText(answer);
                finishJob(test?"Test local réussi.":"Génération terminée · modèle déchargé.");});}catch(Exception e){runOnUiThread(()->finishJob("Génération : "+e.getMessage()));}});}
    private void finishJob(String s){if(isDestroyed())return;busy=false;state.setText(s);refresh();}
    @Override protected void onDestroy(){cancelled.set(true);if(nativeReady)stopNative();if(player!=null)player.release();if(videoPreview!=null)videoPreview.stopPlayback();worker.shutdownNow();super.onDestroy();}
}
